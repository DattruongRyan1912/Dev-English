package ai

import "testing"

func TestNewAzureSpeechFromEnvBuildsRegionalEndpoints(t *testing.T) {
	t.Setenv("AZURE_SPEECH_KEY", "azure-key")
	t.Setenv("AZURE_SPEECH_REGION", "southeastasia")
	t.Setenv("AZURE_SPEECH_STT_BASE_URL", "")
	t.Setenv("AZURE_SPEECH_TTS_BASE_URL", "")

	provider := NewAzureSpeechFromEnv()
	if provider.STTURL != "https://southeastasia.stt.speech.microsoft.com" {
		t.Fatalf("unexpected STT endpoint: %q", provider.STTURL)
	}
	if provider.TTSURL != "https://southeastasia.tts.speech.microsoft.com" {
		t.Fatalf("unexpected TTS endpoint: %q", provider.TTSURL)
	}
	if !provider.Configured() {
		t.Fatal("provider should be configured")
	}
}
