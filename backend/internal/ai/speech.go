package ai

import (
	"bytes"
	"context"
	"encoding/base64"
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
	header.Set("Content-Disposition", `form-data; name="file"; filename="recording"`)
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
	return strings.TrimSpace(p.APIKey) != "" && strings.TrimSpace(p.STTURL) != ""
}

func (p *AzureSpeechProvider) Assess(ctx context.Context, audio []byte, mimeType, reference string) (PronunciationResult, error) {
	if !p.Configured() {
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
	if !p.Configured() {
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

func providerHTTPError(name string, resp *http.Response) error {
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return fmt.Errorf("%s returned HTTP %d: %s", name, resp.StatusCode, strings.TrimSpace(string(payload)))
}
