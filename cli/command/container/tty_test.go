package container

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/internal/test"
	"github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestInitTtySizeErrors(t *testing.T) {
	expectedError := "failed to resize tty, using default size\n"
	fakeContainerExecResizeFunc := func(id string, options container.ResizeOptions) error {
		return errors.New("Error response from daemon: no such exec")
	}
	fakeResizeTtyFunc := func(ctx context.Context, cli command.Cli, id string, isExec bool) error {
		height, width := uint(1024), uint(768)
		return resizeTtyTo(ctx, cli.Client(), id, height, width, isExec)
	}
	ctx := context.Background()
	cli := test.NewFakeCli(&fakeClient{containerExecResizeFunc: fakeContainerExecResizeFunc})
	initTtySize(ctx, cli, "8mm8nn8tt8bb", true, fakeResizeTtyFunc)
	time.Sleep(1500 * time.Millisecond)
	assert.Check(t, is.Equal(expectedError, cli.ErrBuffer().String()))
}
