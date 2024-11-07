package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
)

// ModelStatus represents the status information for each model under an orchestrator.
type ModelStatus struct {
	Cold int `json:"Cold"`
	Warm int `json:"Warm"`
}

// ModelInfo represents information about a model in a pipeline.
type ModelInfo struct {
	Name   string      `json:"name"`
	Status ModelStatus `json:"status"`
}

// PipelineInfo represents information about a pipeline.
type PipelineInfo struct {
	Type   string      `json:"type"`
	Models []ModelInfo `json:"models"`
}

// OrchestratorInfo represents the information for each orchestrator.
type OrchestratorInfo struct {
	Address   string         `json:"address"`
	Pipelines []PipelineInfo `json:"pipelines"`
}

// OrchestratorAICapabilitiesManager represents the full structure with better type safety.
type OrchestratorAICapabilitiesManager struct {
	Orchestrators []OrchestratorInfo `json:"orchestrators"`
}

// NewOrchestratorAICapabilitiesManager creates and initializes a new OrchestratorAICapabilitiesManager structure.
func NewOrchestratorAICapabilitiesManager() *OrchestratorAICapabilitiesManager {
	return &OrchestratorAICapabilitiesManager{
		Orchestrators: []OrchestratorInfo{},
	}
}

// AddOrchestrator adds a new orchestrator if it doesn't exist.
func (ncm *OrchestratorAICapabilitiesManager) AddOrchestrator(orchAddress string) {
	if ncm.getOrchestrator(orchAddress) == nil {
		ncm.Orchestrators = append(ncm.Orchestrators, OrchestratorInfo{
			Address:   orchAddress,
			Pipelines: []PipelineInfo{},
		})
	}
}

// getOrchestrator retrieves an orchestrator by address.
func (ncm *OrchestratorAICapabilitiesManager) getOrchestrator(orchAddress string) *OrchestratorInfo {
	for i := range ncm.Orchestrators {
		if ncm.Orchestrators[i].Address == orchAddress {
			return &ncm.Orchestrators[i]
		}
	}
	return nil
}

// AddOrchestratorPipeline adds or updates a pipeline entry for an orchestrator.
func (ncm *OrchestratorAICapabilitiesManager) AddOrchestratorPipeline(orchAddress string, pipelineType string) {
	orch := ncm.getOrchestrator(orchAddress)
	if orch == nil {
		ncm.AddOrchestrator(orchAddress)
		orch = ncm.getOrchestrator(orchAddress)
	}

	if orch.getPipeline(pipelineType) == nil {
		orch.Pipelines = append(orch.Pipelines, PipelineInfo{
			Type:   pipelineType,
			Models: []ModelInfo{},
		})
	}
}

// getPipeline retrieves a pipeline by type.
func (orch *OrchestratorInfo) getPipeline(pipelineType string) *PipelineInfo {
	for i := range orch.Pipelines {
		if orch.Pipelines[i].Type == pipelineType {
			return &orch.Pipelines[i]
		}
	}
	return nil
}

// AddOrchestratorPipelineModel adds or updates a model in an orchestrator's pipeline.
func (ncm *OrchestratorAICapabilitiesManager) AddOrchestratorPipelineModel(orchAddress, pipelineType, modelName string, warm bool) error {
	orch := ncm.getOrchestrator(orchAddress)
	if orch == nil {
		return errors.New("orchestrator not found")
	}
	pipeline := orch.getPipeline(pipelineType)
	if pipeline == nil {
		orch.Pipelines = append(orch.Pipelines, PipelineInfo{
			Type:   pipelineType,
			Models: []ModelInfo{},
		})
		pipeline = orch.getPipeline(pipelineType)
	}

	model := pipeline.getModel(modelName)
	if model == nil {
		pipeline.Models = append(pipeline.Models, ModelInfo{
			Name:   modelName,
			Status: ModelStatus{},
		})
		model = pipeline.getModel(modelName)
	}

	if warm {
		model.Status.Warm++
	} else {
		model.Status.Cold++
	}
	return nil
}

// getModel retrieves a model by name.
func (pipeline *PipelineInfo) getModel(modelName string) *ModelInfo {
	for i := range pipeline.Models {
		if pipeline.Models[i].Name == modelName {
			return &pipeline.Models[i]
		}
	}
	return nil
}

// PrintJSONData is a utility function to print the current JSONData.
func (ncm *OrchestratorAICapabilitiesManager) PrintJSONData() {
	fmt.Printf("Orchestrators: %+v\n", ncm.Orchestrators)
}

func buildOrchestratorAICapabilitiesManager(livepeerNode *core.LivepeerNode) (*OrchestratorAICapabilitiesManager, error) {
	caps := core.NewCapabilities(core.DefaultCapabilities(), nil)
	caps.SetPerCapabilityConstraints(nil)
	ods, err := livepeerNode.OrchestratorPool.GetOrchestrators(context.Background(), 100, newSuspender(), caps, common.ScoreAtLeast(0))
	caps.SetMinVersionConstraint(livepeerNode.Capabilities.MinVersionConstraint())
	if err != nil {
		return nil, err
	}

	networkCapsMgr := NewOrchestratorAICapabilitiesManager()
	remoteInfos := ods.GetRemoteInfos()

	glog.V(common.SHORT).Infof("getting network capabilities for %d orchestrators", len(remoteInfos))

	for idx, orch_info := range remoteInfos {
		glog.V(common.DEBUG).Infof("getting capabilities for orchestrator %d %v", idx, orch_info.Transcoder)

		// Ensure the orch has the proper on-chain TicketParams to ensure ethAddress was configured.
		tp := orch_info.TicketParams
		var orchAddr string
		if tp != nil {
			ethAddress := tp.Recipient
			orchAddr = hexutil.Encode(ethAddress)
		} else {
			orchAddr = "0x0000000000000000000000000000000000000000"
		}

		// Parse the capabilities and capacities.
		if orch_info.GetCapabilities() != nil {
			for capability, constraints := range orch_info.Capabilities.Constraints.PerCapability {
				capName, err := core.CapabilityToName(core.Capability(int(capability)))
				if err != nil {
					continue
				}
				networkCapsMgr.AddOrchestratorPipeline(orchAddr, capName)

				models := constraints.GetModels()
				for model, constraint := range models {
					err := networkCapsMgr.AddOrchestratorPipelineModel(orchAddr, capName, model, constraint.GetWarm())
					if err != nil {
						glog.V(common.DEBUG).Infof("  error adding model to orch %v", err)
					}
				}
			}
		}
	}
	return networkCapsMgr, nil
}
