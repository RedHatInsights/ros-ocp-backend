package services

import (
	"github.com/redhatinsights/ros-ocp-backend/internal/types/kruizePayload"
	namespacePayload "github.com/redhatinsights/ros-ocp-backend/internal/types/kruizePayload/namespace"
)

type AcceptedPayloadType interface {
	kruizePayload.UpdateResult | namespacePayload.UpdateNamespaceResult
}

func SliceMetricsUpdatePayloadToChunks[T AcceptedPayloadType](objects []T) [][]T {
	var chunks [][]T
	chunkSize := cfg.KruizeMaxBulkChunkSize
	for i := 0; i < len(objects); i += chunkSize {
		end := min(i+chunkSize, len(objects))
		chunks = append(chunks, objects[i:end])
	}
	return chunks
}
