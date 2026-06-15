package syncmap

import (
	"encoding/binary"
	"errors"
	"math"
)

const EnvelopeRate = 100

func PCM16LEToRMSEnvelope(pcm []byte, sampleRate int) ([]float64, error) {
	if sampleRate <= 0 {
		return nil, errors.New("sample rate must be positive")
	}
	if len(pcm) < 2 {
		return nil, errors.New("pcm buffer is empty")
	}
	samples := len(pcm) / 2
	frame := sampleRate / 50
	hop := sampleRate / EnvelopeRate
	if frame <= 0 || hop <= 0 || samples < frame {
		return nil, errors.New("pcm buffer too short for envelope window")
	}
	env := []float64{}
	for start := 0; start+frame <= samples; start += hop {
		sum := 0.0
		for i := start; i < start+frame; i++ {
			v := float64(int16(binary.LittleEndian.Uint16(pcm[i*2:i*2+2]))) / 32768.0
			sum += v * v
		}
		env = append(env, math.Sqrt(sum/float64(frame)))
	}
	return Normalize(env)
}

func Normalize(values []float64) ([]float64, error) {
	if len(values) == 0 {
		return nil, errors.New("no values to normalize")
	}
	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))
	variance := 0.0
	for _, v := range values {
		d := v - mean
		variance += d * d
	}
	std := math.Sqrt(variance / float64(len(values)))
	if std < 1e-9 {
		return nil, errors.New("mostly silent or flat envelope")
	}
	out := make([]float64, len(values))
	for i, v := range values {
		out[i] = (v - mean) / std
	}
	return out, nil
}
