package docker

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type Handler struct {
	client *client.Client
}

type containerResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Image  string `json:"image"`
	State  string `json:"state"`
	Status string `json:"status"`
}

type containerStatsResponse struct {
	ContainerID   string  `json:"container_id"`
	ContainerName string  `json:"container_name"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsage   uint64  `json:"memory_usage"`
	MemoryLimit   uint64  `json:"memory_limit"`
	MemoryPercent float64 `json:"memory_percent"`
}

func NewHandler(dockerClient *client.Client) *Handler {
	return &Handler{
		client: dockerClient,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /containers", h.listContainers)
	mux.HandleFunc("GET /containers/{id}/stats", h.containerStats)
}

func (h *Handler) listContainers(w http.ResponseWriter, r *http.Request) {
	result, err := h.client.ContainerList(
		r.Context(),
		client.ContainerListOptions{
			All: true,
		},
	)
	if err != nil {
		http.Error(
			w,
			"failed to list Docker containers",
			http.StatusInternalServerError,
		)

		log.Printf("failed to list containers: %v", err)
		return
	}

	containers := make(
		[]containerResponse,
		0,
		len(result.Items),
	)

	for _, currentContainer := range result.Items {
		name := ""

		if len(currentContainer.Names) > 0 {
			name = strings.TrimPrefix(
				currentContainer.Names[0],
				"/",
			)
		}

		id := currentContainer.ID

		if len(id) > 12 {
			id = id[:12]
		}

		containers = append(
			containers,
			containerResponse{
				ID:     id,
				Name:   name,
				Image:  currentContainer.Image,
				State:  string(currentContainer.State),
				Status: currentContainer.Status,
			},
		)
	}

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	if err := json.NewEncoder(w).Encode(containers); err != nil {
		log.Printf(
			"failed to encode containers: %v",
			err,
		)
	}
}

func (h *Handler) containerStats(w http.ResponseWriter, r *http.Request) {
	containerID := r.PathValue("id")

	result, err := h.client.ContainerStats(
		r.Context(),
		containerID,
		client.ContainerStatsOptions{
			Stream:                false,
			IncludePreviousSample: true,
		},
	)
	if err != nil {
		http.Error(
			w,
			"failed to get container stats",
			http.StatusInternalServerError,
		)

		log.Printf(
			"failed to get stats for container %s: %v",
			containerID,
			err,
		)

		return
	}

	defer result.Body.Close()

	var stats container.StatsResponse

	if err := json.NewDecoder(result.Body).Decode(&stats); err != nil {
		http.Error(
			w,
			"failed to decode container stats",
			http.StatusInternalServerError,
		)

		log.Printf(
			"failed to decode stats: %v",
			err,
		)

		return
	}

	memoryUsage := calculateMemoryUsage(stats)
	memoryLimit := stats.MemoryStats.Limit

	var memoryPercent float64

	if memoryLimit > 0 {
		memoryPercent =
			(float64(memoryUsage) / float64(memoryLimit)) * 100
	}

	response := containerStatsResponse{
		ContainerID:   stats.ID,
		ContainerName: stats.Name,
		CPUPercent:    calculateCPUPercent(stats),
		MemoryUsage:   memoryUsage,
		MemoryLimit:   memoryLimit,
		MemoryPercent: memoryPercent,
	}

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf(
			"failed to encode container stats: %v",
			err,
		)
	}
}

func calculateCPUPercent(stats container.StatsResponse) float64 {
	cpuDelta := float64(
		stats.CPUStats.CPUUsage.TotalUsage -
			stats.PreCPUStats.CPUUsage.TotalUsage,
	)

	systemDelta := float64(
		stats.CPUStats.SystemUsage -
			stats.PreCPUStats.SystemUsage,
	)

	if cpuDelta <= 0 || systemDelta <= 0 {
		return 0
	}

	cpuCount := float64(stats.CPUStats.OnlineCPUs)

	if cpuCount == 0 {
		cpuCount = float64(
			len(stats.CPUStats.CPUUsage.PercpuUsage),
		)
	}

	if cpuCount == 0 {
		cpuCount = 1
	}

	return (cpuDelta / systemDelta) *
		cpuCount *
		100
}

func calculateMemoryUsage(stats container.StatsResponse) uint64 {
	usage := stats.MemoryStats.Usage

	if inactiveFile, ok :=
		stats.MemoryStats.Stats["inactive_file"]; ok {

		if usage >= inactiveFile {
			return usage - inactiveFile
		}
	}

	if cache, ok :=
		stats.MemoryStats.Stats["cache"]; ok {

		if usage >= cache {
			return usage - cache
		}
	}

	return usage
}
