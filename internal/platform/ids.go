package platform

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

type IDGenerator interface {
	New(prefix string) string
}

type RandomIDGenerator struct{}

func (RandomIDGenerator) New(prefix string) string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		panic(fmt.Errorf("generate random id: %w", err))
	}
	return prefix + "_" + hex.EncodeToString(buffer)
}

type SequenceIDGenerator struct {
	Next int
}

func (g *SequenceIDGenerator) New(prefix string) string {
	g.Next++
	return fmt.Sprintf("%s_%04d", prefix, g.Next)
}
