package google

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	speech "cloud.google.com/go/speech/apiv1"
	"cloud.google.com/go/speech/apiv1/speechpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/agnivade/stt_challenge/providers"
)

const providerName = "google"

const targetSampleRate = 16000

// streamingRecognizeClient is a local interface that wraps the methods we need
// from speechpb.Speech_StreamingRecognizeClient to enable easier testing
type streamingRecognizeClient interface {
	Send(*speechpb.StreamingRecognizeRequest) error
	Recv() (*speechpb.StreamingRecognizeResponse, error)
	CloseSend() error
}

// Provider implements the providers.Provider interface for Google Speech-to-Text API.
type Provider struct {
	client *speech.Client
}

// NewProvider creates a new Google Speech provider with the given client.
func NewProvider(client *speech.Client) *Provider {
	return &Provider{
		client: client,
	}
}

// Name returns the name of the provider.
func (p *Provider) Name() string {
	return providerName
}

// NewSession creates a new Google Speech transcription session.
func (p *Provider) NewSession(ctx context.Context, config providers.SessionConfig) (providers.Session, error) {
	stream, err := p.client.StreamingRecognize(ctx)
	if err != nil {
		return nil, err
	}

	// Send initial configuration
	req := &speechpb.StreamingRecognizeRequest{
		StreamingRequest: &speechpb.StreamingRecognizeRequest_StreamingConfig{
			StreamingConfig: &speechpb.StreamingRecognitionConfig{
				Config: &speechpb.RecognitionConfig{
					Encoding:        speechpb.RecognitionConfig_LINEAR16,
					SampleRateHertz: targetSampleRate,
					LanguageCode:    config.LanguageCode,
				},
				InterimResults: config.InterimResults,
			},
		},
	}

	if err := stream.Send(req); err != nil {
		stream.CloseSend()
		return nil, fmt.Errorf("error while sending RecognitionConfig: %w", err)
	}

	return &Session{
		stream:     stream,
		ctx:        ctx,
		sampleRate: config.SampleRate,
	}, nil
}

// Session implements the providers.Session interface for Google Speech-to-Text API.
type Session struct {
	stream     streamingRecognizeClient
	ctx        context.Context
	sampleRate int
}

// SendAudio sends audio data to the Google Speech stream.
func (s *Session) SendAudio(audioData []byte) error {
	if s.sampleRate != targetSampleRate {
		// resample to 16000
		var err error
		audioData, err = resample(audioData, s.sampleRate, targetSampleRate)
		if err != nil {
			return fmt.Errorf("error in resampling audio: %w", err)
		}
	}
	req := &speechpb.StreamingRecognizeRequest{
		StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
			AudioContent: audioData,
		},
	}
	return s.stream.Send(req)
}

// ReceiveTranscription receives the next transcription result from the Google Speech stream.
// Returns immediately for both interim and final results so words appear as they are spoken.
// Returns io.EOF when the stream is closed.
func (s *Session) ReceiveTranscription() (providers.TranscriptionResult, error) {
	for {
		resp, err := s.stream.Recv()
		if errors.Is(err, io.EOF) || status.Code(err) == codes.Canceled {
			return providers.TranscriptionResult{}, io.EOF
		}
		if err != nil {
			return providers.TranscriptionResult{}, fmt.Errorf("error while receiving transcription: %w", err)
		}

		for _, result := range resp.Results {
			if len(result.Alternatives) == 0 {
				continue
			}
			alt := result.Alternatives[0]
			return providers.TranscriptionResult{
				Text:         alt.Transcript,
				IsFinal:      result.IsFinal,
				Confidence:   alt.Confidence,
				ProviderName: providerName,
				ReceivedAt:   time.Now(),
			}, nil
		}
		// resp had no usable alternatives — fetch the next response
	}
}

// Close closes the Google Speech stream.
func (s *Session) Close() error {
	return s.stream.CloseSend()
}
