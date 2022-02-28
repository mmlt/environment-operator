package addon

import (
	"context"
	"os/exec"
	"time"
)

// AddonFake provides an Addonr for testing.
type AddonFake struct {
	// Tally is the number of times Start has been called.
	StartTally int

	// Result that is played back by the fake implementation of Start.
	StartResult []KTResult
}

func (a *AddonFake) Start(ctx context.Context, env []string, dir, jobPath, valuesPath, kubeconfigPath, masterVaultPath string) (*exec.Cmd, chan KTResult, error) {
	a.StartTally++

	out := make(chan KTResult)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for _, v := range a.StartResult {
			select {
			case <-ticker.C:
				out <- v
			case <-ctx.Done():
				return
			}
		}
		close(out)
	}()

	return nil, out, nil
}

// SetupFakeResultsForCreate sets-up the receiver with data that is returned during testing.
func (a *AddonFake) SetupFakeResult() {
	a.StartResult = []KTResult{
		{Added: 0, Changed: 1, Deleted: 0, Errors: []string(nil), Object: "namespace/kube-system unchanged", ObjectID: "1", Action: "apply"},
		{Added: 0, Changed: 2, Deleted: 0, Errors: []string(nil), Object: "namespace/xyz-system created", ObjectID: "2", Action: "apply"},
		{Added: 0, Changed: 3, Deleted: 0, Errors: []string(nil), Object: "pod/opa-5cd59b58bc-rrrxf condition met", ObjectID: "3", Action: "wait"},
	}
}
