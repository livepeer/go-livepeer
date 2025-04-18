package worker

import "testing"

func TestLowestVersion(t *testing.T) {
	versions := []Version{
		{Pipeline: "pipeline1", ModelId: "model1", Version: "1.0.0"},
		{Pipeline: "pipeline1", ModelId: "model1", Version: "2.0.0"},
		{Pipeline: "pipeline2", ModelId: "model2", Version: "1.5.0"},
		{Pipeline: "pipeline2", ModelId: "model2", Version: "0.1.0"},
		{Pipeline: "pipeline2", ModelId: "model2", Version: "1.5.0"},
		{Pipeline: "pipeline2", ModelId: "model2", Version: "0.0.2"},
	}

	tests := []struct {
		pipeline string
		modelId  string
		expected string
	}{
		{"pipeline1", "model1", "1.0.0"},
		{"pipeline2", "model2", "0.0.2"},
		{"pipeline3", "model3", ""},
	}

	for _, test := range tests {
		t.Run(test.pipeline+"_"+test.modelId, func(t *testing.T) {
			result := LowestVersion(versions, test.pipeline, test.modelId)
			if result != test.expected {
				t.Errorf("Expected %s, got %s", test.expected, result)
			}
		})
	}
}
