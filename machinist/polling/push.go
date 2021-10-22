package polling

import (
	"context"
	"github.com/enfabrica/enkit/machinist/rpc/machinist"
	"github.com/enfabrica/enkit/machinist/state"
)

func PushState(ctx context.Context, client machinist.UserplaneStateClient, controller *state.MachineController) error {
	r := machinist.StateForwardRequest{
		Machines: controller.Machines,
	}
	_, err := client.ExportState(ctx, &r)
	return err
}
