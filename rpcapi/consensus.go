package rpcapi

import (
	"github.com/fractalplatform/fractal/plugin"
)

type ConsensusAPI struct {
	b Backend
}

func NewConsensusAPI(b Backend) *ConsensusAPI {
	return &ConsensusAPI{b}
}

func (api *ConsensusAPI) GetAllCandidates() ([]string, error) {
	pm, err := api.b.GetPM()
	if err != nil {
		return nil, err
	}
	return pm.GetAllCandidates(), nil
}

func (api *ConsensusAPI) GetCandidateInfo(account string) (*plugin.CandidateInfo, error) {
	pm, err := api.b.GetPM()
	if err != nil {
		return nil, err
	}
	return pm.GetCandidateInfo(account), nil
}
