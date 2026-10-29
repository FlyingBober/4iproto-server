package traffic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"Abdal_4iProto_Server/core/config"
)

func TestAtomicWriteJSON(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "atomic_traffic_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	targetFile := filepath.Join(tempDir, "test_traffic.json")
	testData := config.TrafficStats{
		Username:           "test_user",
		TotalBytesSent:     1024,
		TotalBytesReceived: 2048,
		TotalBytes:         3072,
	}

	if err := atomicWriteJSON(targetFile, testData); err != nil {
		t.Fatalf("atomicWriteJSON failed: %v", err)
	}

	// Verify file content
	data, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	var readData config.TrafficStats
	if err := json.Unmarshal(data, &readData); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}

	if readData.TotalBytes != testData.TotalBytes || readData.Username != testData.Username {
		t.Errorf("read data %+v does not match expected %+v", readData, testData)
	}

	// Verify overwrite is atomic and works
	testData.TotalBytesSent = 4096
	testData.TotalBytes = 6144
	if err := atomicWriteJSON(targetFile, testData); err != nil {
		t.Fatalf("atomicWriteJSON overwrite failed: %v", err)
	}

	data2, _ := os.ReadFile(targetFile)
	var readData2 config.TrafficStats
	_ = json.Unmarshal(data2, &readData2)
	if readData2.TotalBytes != 6144 {
		t.Errorf("expected updated TotalBytes=6144, got %d", readData2.TotalBytes)
	}
}

func TestStore_UpdateAndSave(t *testing.T) {
	store := NewStore()

	exceeded := store.UpdateRealTime("user_alpha", "1.2.3.4", 500, 500, 100)
	if exceeded {
		t.Error("expected 1000 bytes to not exceed 100MB limit")
	}

	statsAny, ok := store.trafficMap.Load("user_alpha")
	if !ok {
		t.Fatal("expected user_alpha stats in map")
	}
	stats := statsAny.(*config.TrafficStats)
	if stats.TotalBytes != 1000 {
		t.Errorf("expected TotalBytes=1000, got %d", stats.TotalBytes)
	}

	// Exceed limit test
	exceeded2 := store.UpdateRealTime("user_alpha", "1.2.3.4", 200*1024*1024, 0, 100)
	if !exceeded2 {
		t.Error("expected 200MB to exceed 100MB limit")
	}
}
