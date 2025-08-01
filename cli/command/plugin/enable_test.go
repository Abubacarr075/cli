package plugin

import (
	"errors"
	"io"
	"testing"

	"github.com/docker/cli/internal/test"
	"github.com/moby/moby/client"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestPluginEnableErrors(t *testing.T) {
	testCases := []struct {
		args             []string
		flags            map[string]string
		pluginEnableFunc func(name string, options client.PluginEnableOptions) error
		expectedError    string
	}{
		{
			args:          []string{},
			expectedError: "requires 1 argument",
		},
		{
			args:          []string{"too-many", "arguments"},
			expectedError: "requires 1 argument",
		},
		{
			args: []string{"plugin-foo"},
			pluginEnableFunc: func(name string, options client.PluginEnableOptions) error {
				return errors.New("failed to enable plugin")
			},
			expectedError: "failed to enable plugin",
		},
		{
			args: []string{"plugin-foo"},
			flags: map[string]string{
				"timeout": "-1",
			},
			expectedError: "negative timeout -1 is invalid",
		},
	}

	for _, tc := range testCases {
		cmd := newEnableCommand(test.NewFakeCli(&fakeClient{
			pluginEnableFunc: tc.pluginEnableFunc,
		}))
		cmd.SetArgs(tc.args)
		for key, value := range tc.flags {
			assert.NilError(t, cmd.Flags().Set(key, value))
		}
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		assert.ErrorContains(t, cmd.Execute(), tc.expectedError)
	}
}

func TestPluginEnable(t *testing.T) {
	cli := test.NewFakeCli(&fakeClient{
		pluginEnableFunc: func(name string, options client.PluginEnableOptions) error {
			return nil
		},
	})

	cmd := newEnableCommand(cli)
	cmd.SetArgs([]string{"plugin-foo"})
	assert.NilError(t, cmd.Execute())
	assert.Check(t, is.Equal("plugin-foo\n", cli.OutBuffer().String()))
}
