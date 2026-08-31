package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strings"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/connectors"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
)

const maxAudioBytes = 25 << 20

type GroqSTTProvider struct {
	APIKey  string
	BaseURL string
	Model   string
	Client  *http.Client
}

func NewGroqSTTFromEnv() *GroqSTTProvider {
	model := os.Getenv("GROQ_STT_MODEL")
	if model == "" {
		model = "whisper-large-v3"
	}
	baseURL := strings.TrimRight(os.Getenv("GROQ_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = "https://api.groq.com/openai/v1"
	}
	return &GroqSTTProvider{APIKey: os.Getenv("GROQ_API_KEY"), BaseURL: baseURL, Model: model, Client: &http.Client{Timeout: 90 * time.Second}}
}

func (p *GroqSTTProvider) Name() string     { return "groq-whisper-large-v3" }
func (p *GroqSTTProvider) Configured() bool { return strings.TrimSpace(p.APIKey) != "" }

func (p *GroqSTTProvider) HealthCheck(ctx context.Context) error {
	if !p.Configured() {
		return ErrProviderUnavailable
	}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(p.BaseURL, "/")+"/models", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return providerHTTPError("groq health check", resp)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return nil
}

func (p *GroqSTTProvider) ProbeCapability(ctx context.Context, capability string) domain.ProviderCheck {
	check := startProbe("Groq Whisper", capability, p.Model, p.Configured())
	if !check.Configured {
		return check
	}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(p.BaseURL, "/")+"/models", nil)
	if err != nil {
		return finishProbeError(check, err)
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	check, resp := doProbeRequest(check, client, req)
	if resp == nil || !check.Healthy {
		return check
	}
	defer resp.Body.Close()
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return finishProbeError(check, err)
	}
	for _, item := range payload.Data {
		if item.ID == p.Model {
			return check
		}
	}
	check.Healthy = false
	check.Status = "unhealthy"
	check.Error = "model_unavailable"
	return check
}

func (p *GroqSTTProvider) Transcribe(ctx context.Context, audio []byte, mimeType string) (Transcript, error) {
	if !p.Configured() {
		return Transcript{}, ErrProviderUnavailable
	}
	if len(audio) == 0 || len(audio) > maxAudioBytes {
		return Transcript{}, errors.New("audio must be between 1 byte and 25 MB")
	}
	if mimeType == "" {
		mimeType = "audio/webm"
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	extension := "webm"
	switch strings.ToLower(strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0])) {
	case "audio/wav", "audio/x-wav", "audio/wave":
		extension = "wav"
	case "audio/mpeg", "audio/mp3":
		extension = "mp3"
	case "audio/mp4", "audio/m4a":
		extension = "m4a"
	case "audio/ogg":
		extension = "ogg"
	case "audio/flac":
		extension = "flac"
	case "audio/opus":
		extension = "opus"
	}
	header.Set("Content-Disposition", `form-data; name="file"; filename="recording.`+extension+`"`)
	header.Set("Content-Type", mimeType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return Transcript{}, err
	}
	if _, err := part.Write(audio); err != nil {
		return Transcript{}, err
	}
	if err := writer.WriteField("model", p.Model); err != nil {
		return Transcript{}, err
	}
	if err := writer.WriteField("language", "en"); err != nil {
		return Transcript{}, err
	}
	if err := writer.Close(); err != nil {
		return Transcript{}, err
	}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(p.BaseURL, "/")+"/audio/transcriptions", &body)
	if err != nil {
		return Transcript{}, err
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := client.Do(req)
	if err != nil {
		return Transcript{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Transcript{}, providerHTTPError("groq", resp)
	}
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return Transcript{}, err
	}
	if strings.TrimSpace(payload.Text) == "" {
		return Transcript{}, errors.New("groq returned an empty transcript")
	}
	return Transcript{Text: strings.TrimSpace(payload.Text), Confidence: 0.85}, nil
}

type AzureSpeechProvider struct {
	APIKey string
	Region string
	STTURL string
	TTSURL string
	Voice  string
	Client *http.Client
}

func NewAzureSpeechFromEnv() *AzureSpeechProvider {
	region := strings.TrimSpace(os.Getenv("AZURE_SPEECH_REGION"))
	sttBase := strings.TrimRight(strings.TrimSpace(os.Getenv("AZURE_SPEECH_STT_BASE_URL")), "/")
	ttsBase := strings.TrimRight(strings.TrimSpace(os.Getenv("AZURE_SPEECH_TTS_BASE_URL")), "/")
	if sttBase == "" && region != "" {
		sttBase = "https://" + region + ".stt.speech.microsoft.com"
	}
	if ttsBase == "" && region != "" {
		ttsBase = "https://" + region + ".tts.speech.microsoft.com"
	}
	voice := os.Getenv("AZURE_TTS_VOICE")
	if voice == "" {
		voice = "en-US-JennyNeural"
	}
	return &AzureSpeechProvider{APIKey: os.Getenv("AZURE_SPEECH_KEY"), Region: region, STTURL: sttBase, TTSURL: ttsBase, Voice: voice, Client: &http.Client{Timeout: 90 * time.Second}}
}

func (p *AzureSpeechProvider) Name() string {
	return "azure-speech"
}

func (p *AzureSpeechProvider) Configured() bool {
	return strings.TrimSpace(p.APIKey) != "" && (strings.TrimSpace(p.STTURL) != "" || strings.TrimSpace(p.TTSURL) != "")
}

func (p *AzureSpeechProvider) PronunciationConfigured() bool {
	return p.pronunciationConfigured()
}

func (p *AzureSpeechProvider) pronunciationConfigured() bool {
	return strings.TrimSpace(p.APIKey) != "" && strings.TrimSpace(p.STTURL) != ""
}

func (p *AzureSpeechProvider) TTSConfigured() bool {
	return p.ttsConfigured()
}

func (p *AzureSpeechProvider) ttsConfigured() bool {
	return strings.TrimSpace(p.APIKey) != "" && strings.TrimSpace(p.TTSURL) != ""
}

func (p *AzureSpeechProvider) HealthCheck(ctx context.Context) error {
	if !p.ttsConfigured() {
		return ErrProviderUnavailable
	}
	baseURL := strings.TrimRight(p.TTSURL, "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(p.STTURL, "/")
	}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/cognitiveservices/voices/list", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", p.APIKey)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return providerHTTPError("azure speech health check", resp)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return nil
}

func (p *AzureSpeechProvider) ProbeCapability(ctx context.Context, capability string) domain.ProviderCheck {
	switch capability {
	case "pronunciation_assessment":
		return p.probePronunciation(ctx, capability)
	case "text_to_speech":
		return p.probeTTS(ctx, capability)
	default:
		return domain.ProviderCheck{Provider: "Azure Speech", Capability: capability, Status: "unhealthy", Error: "unsupported_capability"}
	}
}

func (p *AzureSpeechProvider) probeTTS(ctx context.Context, capability string) domain.ProviderCheck {
	check := startProbe("Azure Neural TTS", capability, p.Voice, p.ttsConfigured())
	if !check.Configured {
		return check
	}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(p.TTSURL, "/")+"/cognitiveservices/voices/list", nil)
	if err != nil {
		return finishProbeError(check, err)
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", p.APIKey)
	check, resp := doProbeRequest(check, client, req)
	if resp == nil || !check.Healthy {
		return check
	}
	defer resp.Body.Close()
	var voices []struct {
		ShortName string `json:"ShortName"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&voices); err != nil {
		return finishProbeError(check, err)
	}
	for _, voice := range voices {
		if voice.ShortName == p.Voice {
			return check
		}
	}
	check.Healthy = false
	check.Status = "unhealthy"
	check.Error = "voice_unavailable"
	return check
}

func (p *AzureSpeechProvider) probePronunciation(ctx context.Context, capability string) domain.ProviderCheck {
	check := startProbe("Azure Pronunciation", capability, "azure-pronunciation-assessment", p.pronunciationConfigured())
	if !check.Configured {
		return check
	}
	assessmentPayload, err := json.Marshal(map[string]string{"ReferenceText": "test", "GradingSystem": "HundredMark", "Granularity": "Word", "Dimension": "Comprehensive"})
	if err != nil {
		return finishProbeError(check, err)
	}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	url := strings.TrimRight(p.STTURL, "/") + "/speech/recognition/conversation/cognitiveservices/v1?language=en-US&format=detailed"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(silentWAV()))
	if err != nil {
		return finishProbeError(check, err)
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", p.APIKey)
	req.Header.Set("Content-Type", "audio/wav")
	req.Header.Set("Pronunciation-Assessment", base64.StdEncoding.EncodeToString(assessmentPayload))
	req.Header.Set("Accept", "application/json")
	check, resp := doProbeRequest(check, client, req)
	if resp != nil {
		resp.Body.Close()
	}
	return check
}

func (p *AzureSpeechProvider) Assess(ctx context.Context, audio []byte, mimeType, reference string) (PronunciationResult, error) {
	if !p.pronunciationConfigured() {
		return PronunciationResult{}, ErrProviderUnavailable
	}
	if len(audio) == 0 || len(audio) > maxAudioBytes {
		return PronunciationResult{}, errors.New("audio must be between 1 byte and 25 MB")
	}
	if reference == "" {
		return PronunciationResult{}, errors.New("reference text is required")
	}
	assessmentPayload, err := json.Marshal(map[string]string{"ReferenceText": reference, "GradingSystem": "HundredMark", "Granularity": "Word", "Dimension": "Comprehensive"})
	if err != nil {
		return PronunciationResult{}, err
	}
	assessment := base64.StdEncoding.EncodeToString(assessmentPayload)
	url := strings.TrimRight(p.STTURL, "/") + "/speech/recognition/conversation/cognitiveservices/v1?language=en-US&format=detailed"
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(audio))
	if err != nil {
		return PronunciationResult{}, err
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", p.APIKey)
	req.Header.Set("Content-Type", audioContentType(mimeType))
	req.Header.Set("Pronunciation-Assessment", assessment)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return PronunciationResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return PronunciationResult{}, providerHTTPError("azure pronunciation", resp)
	}
	var payload struct {
		NBest []struct {
			Assessment struct {
				Accuracy     float64 `json:"AccuracyScore"`
				Fluency      float64 `json:"FluencyScore"`
				Completeness float64 `json:"CompletenessScore"`
				Prosody      float64 `json:"ProsodyScore"`
			} `json:"PronunciationAssessment"`
			Words []struct {
				Word       string `json:"Word"`
				Assessment struct {
					Accuracy float64 `json:"AccuracyScore"`
				} `json:"PronunciationAssessment"`
			} `json:"Words"`
		} `json:"NBest"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&payload); err != nil {
		return PronunciationResult{}, err
	}
	if len(payload.NBest) == 0 {
		return PronunciationResult{}, errors.New("azure pronunciation returned no assessment")
	}
	best := payload.NBest[0]
	result := PronunciationResult{Accuracy: best.Assessment.Accuracy, Fluency: best.Assessment.Fluency, Completeness: best.Assessment.Completeness, Prosody: best.Assessment.Prosody, Words: map[string]float64{}}
	for _, word := range best.Words {
		result.Words[word.Word] = word.Assessment.Accuracy
	}
	return result, nil
}

func (p *AzureSpeechProvider) Synthesize(ctx context.Context, text, voice string) ([]byte, error) {
	if !p.ttsConfigured() {
		return nil, ErrProviderUnavailable
	}
	text = strings.TrimSpace(text)
	if text == "" || len([]rune(text)) > 2000 {
		return nil, errors.New("tts text must contain between 1 and 2000 characters")
	}
	if voice == "" {
		voice = p.Voice
	}
	ssml := fmt.Sprintf(`<speak version="1.0" xml:lang="en-US"><voice name="%s">%s</voice></speak>`, html.EscapeString(voice), html.EscapeString(text))
	url := strings.TrimRight(p.TTSURL, "/") + "/cognitiveservices/v1"
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(ssml))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", p.APIKey)
	req.Header.Set("Content-Type", "application/ssml+xml")
	req.Header.Set("X-Microsoft-OutputFormat", "audio-24khz-48kbitrate-mono-mp3")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, providerHTTPError("azure tts", resp)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 10<<20))
}

func audioContentType(value string) string {
	if strings.HasPrefix(strings.ToLower(value), "audio/") {
		return value
	}
	return "audio/wav"
}

func silentWAV() []byte {
	const sampleRate = 16000
	const channels = 1
	const bitsPerSample = 16
	const sampleCount = sampleRate / 10
	dataSize := sampleCount * channels * bitsPerSample / 8
	wav := make([]byte, 44+dataSize)
	copy(wav[0:4], "RIFF")
	binary.LittleEndian.PutUint32(wav[4:8], uint32(len(wav)-8))
	copy(wav[8:12], "WAVE")
	copy(wav[12:16], "fmt ")
	binary.LittleEndian.PutUint32(wav[16:20], 16)
	binary.LittleEndian.PutUint16(wav[20:22], 1)
	binary.LittleEndian.PutUint16(wav[22:24], channels)
	binary.LittleEndian.PutUint32(wav[24:28], sampleRate)
	binary.LittleEndian.PutUint32(wav[28:32], sampleRate*channels*bitsPerSample/8)
	binary.LittleEndian.PutUint16(wav[32:34], channels*bitsPerSample/8)
	binary.LittleEndian.PutUint16(wav[34:36], bitsPerSample)
	copy(wav[36:40], "data")
	binary.LittleEndian.PutUint32(wav[40:44], uint32(dataSize))
	return wav
}

func providerHTTPError(name string, resp *http.Response) error {
	return connectors.NewProviderError(name, "request", resp.StatusCode, "provider request failed")
}
