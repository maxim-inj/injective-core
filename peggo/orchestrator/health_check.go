package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/InjectiveLabs/coretracer"
	gethcommon "github.com/ethereum/go-ethereum/common"
)

const (
	HealthCheckURI = "/health"
)

// Status represents the orchestrator's overall health status with respect to some Peggy network
type Status struct {
	// Nonce of the last Ethereum event confirmed on the Injective chain
	LastObservedEventNonceByNetwork uint64 `json:"last_observed_event_nonce_by_network"`

	// Nonce of the last Ethereum event confirmed by the orchestrator
	LastObservedEventNonceByOrchestrator uint64 `json:"last_observed_event_nonce_by_orchestrator"`

	// Nonce of the (oldest) tx batch this orchestrator needs to sign
	PendingTxBatchToSign uint64 `json:"pending_tx_batch_to_sign"`

	// Nonce(s) of the validator set updates this orchestrator needs to sign
	PendingValidatorSetsToSign []uint64 `json:"pending_validator_sets_to_sign"`

	// Indicator if the running orchestrator is part of the current validator set
	IsPartOfTheCurrentSet bool `json:"is_part_of_the_current_set"`

	// How long the orchestrator has been running
	Uptime string `json:"uptime"`
}

func (s *Orchestrator) RunHealthCheckServer(port uint64) error {
	mux := http.NewServeMux()
	start := time.Now()

	mux.HandleFunc(HealthCheckURI, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		status, err := s.checkHealthStatus(ctx)
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)

			jsonErr := map[string]string{"error": err.Error()}
			_ = json.NewEncoder(w).Encode(jsonErr)
			return
		}

		uptime := time.Since(start)
		status.Uptime = uptime.String()

		w.WriteHeader(http.StatusOK)
		if err = json.NewEncoder(w).Encode(status); err != nil {
			s.logger.Errorln("failed to encode health check response: ", err)
		}
	})

	err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", port), mux)
	if err != nil && errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}

func (s *Orchestrator) checkHealthStatus(ctx context.Context) (Status, error) {
	defer coretracer.Trace(&ctx, s.svcTags)()

	state, err := s.injective.ModuleState(ctx)
	if err != nil {
		coretracer.TraceError(ctx, err)
		return Status{}, fmt.Errorf("failed to get module state: %w", err)
	}

	claim, err := s.injective.LastClaimEventByAddr(ctx, s.cfg.CosmosAddr)
	if err != nil {
		coretracer.TraceError(ctx, err)
		return Status{}, fmt.Errorf("failed to get last claim event: %w", err)
	}

	unsignedBatch, err := s.injective.OldestUnsignedTransactionBatch(ctx, s.cfg.CosmosAddr)
	if err != nil {
		coretracer.TraceError(ctx, err)
		return Status{}, fmt.Errorf("failed to get oldest unsigned batch: %w", err)
	}

	unsignedValsets, err := s.injective.OldestUnsignedValsets(ctx, s.cfg.CosmosAddr)
	if err != nil {
		coretracer.TraceError(ctx, err)
		return Status{}, fmt.Errorf("failed to get oldest unsigned valsets: %w", err)
	}

	vs, err := s.injective.CurrentValset(ctx)
	if err != nil {
		coretracer.TraceError(ctx, err)
		return Status{}, fmt.Errorf("failed to get active validator set on Injective: %w", err)
	}

	bonded := false
	for _, v := range vs.Members {
		if bytes.Equal(s.cfg.EthereumAddr.Bytes(), gethcommon.HexToAddress(v.EthereumAddress).Bytes()) {
			bonded = true
		}
	}

	status := Status{
		LastObservedEventNonceByNetwork:      state.LastObservedNonce,
		LastObservedEventNonceByOrchestrator: claim.EthereumEventNonce,
		IsPartOfTheCurrentSet:                bonded,
	}

	if unsignedBatch != nil {
		status.PendingTxBatchToSign = unsignedBatch.BatchNonce
	}

	for _, vs := range unsignedValsets {
		status.PendingValidatorSetsToSign = append(status.PendingValidatorSetsToSign, vs.Nonce)
	}

	return status, nil
}
