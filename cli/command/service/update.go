package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/docker/cli/cli"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/command/completion"
	"github.com/docker/cli/opts"
	"github.com/docker/cli/opts/swarmopts"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/api/types/versions"
	"github.com/moby/moby/client"
	"github.com/moby/swarmkit/v2/api/defaults"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func newUpdateCommand(dockerCLI command.Cli) *cobra.Command {
	options := newServiceOptions()

	cmd := &cobra.Command{
		Use:   "update [OPTIONS] SERVICE",
		Short: "Update a service",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd.Context(), dockerCLI, cmd.Flags(), options, args[0])
		},
		ValidArgsFunction: completeServiceNames(dockerCLI),
	}

	flags := cmd.Flags()
	flags.String("image", "", "Service image tag")
	flags.Var(&ShlexOpt{}, "args", "Service command args")
	flags.Bool(flagRollback, false, "Rollback to previous specification")
	flags.SetAnnotation(flagRollback, "version", []string{"1.25"})
	flags.Bool("force", false, "Force update even if no changes require it")
	flags.SetAnnotation("force", "version", []string{"1.25"})
	addServiceFlags(flags, options, nil)

	flags.Var(newListOptsVar(), flagEnvRemove, "Remove an environment variable")
	flags.Var(newListOptsVar(), flagGroupRemove, "Remove a previously added supplementary user group from the container")
	flags.SetAnnotation(flagGroupRemove, "version", []string{"1.25"})
	flags.Var(newListOptsVar(), flagLabelRemove, "Remove a label by its key")
	flags.Var(newListOptsVar(), flagContainerLabelRemove, "Remove a container label by its key")
	flags.Var(newListOptsVar(), flagMountRemove, "Remove a mount by its target path")
	// flags.Var(newListOptsVar().WithValidator(validatePublishRemove), flagPublishRemove, "Remove a published port by its target port")
	flags.Var(&swarmopts.PortOpt{}, flagPublishRemove, "Remove a published port by its target port")
	flags.Var(newListOptsVar(), flagConstraintRemove, "Remove a constraint")
	flags.Var(newListOptsVar(), flagDNSRemove, "Remove a custom DNS server")
	flags.SetAnnotation(flagDNSRemove, "version", []string{"1.25"})
	flags.Var(newListOptsVar(), flagDNSOptionRemove, "Remove a DNS option")
	flags.SetAnnotation(flagDNSOptionRemove, "version", []string{"1.25"})
	flags.Var(newListOptsVar(), flagDNSSearchRemove, "Remove a DNS search domain")
	flags.SetAnnotation(flagDNSSearchRemove, "version", []string{"1.25"})
	flags.Var(newListOptsVar(), flagHostRemove, `Remove a custom host-to-IP mapping ("host:ip")`)
	flags.SetAnnotation(flagHostRemove, "version", []string{"1.25"})
	flags.Var(&options.labels, flagLabelAdd, "Add or update a service label")
	flags.Var(&options.containerLabels, flagContainerLabelAdd, "Add or update a container label")
	flags.Var(&options.env, flagEnvAdd, "Add or update an environment variable")
	flags.Var(newListOptsVar(), flagSecretRemove, "Remove a secret")
	flags.SetAnnotation(flagSecretRemove, "version", []string{"1.25"})
	flags.Var(&options.secrets, flagSecretAdd, "Add or update a secret on a service")
	flags.SetAnnotation(flagSecretAdd, "version", []string{"1.25"})

	flags.Var(newListOptsVar(), flagConfigRemove, "Remove a configuration file")
	flags.SetAnnotation(flagConfigRemove, "version", []string{"1.30"})
	flags.Var(&options.configs, flagConfigAdd, "Add or update a config file on a service")
	flags.SetAnnotation(flagConfigAdd, "version", []string{"1.30"})

	flags.Var(&options.mounts, flagMountAdd, "Add or update a mount on a service")
	flags.Var(&options.constraints, flagConstraintAdd, "Add or update a placement constraint")
	flags.Var(&options.placementPrefs, flagPlacementPrefAdd, "Add a placement preference")
	flags.SetAnnotation(flagPlacementPrefAdd, "version", []string{"1.28"})
	flags.Var(&placementPrefOpts{}, flagPlacementPrefRemove, "Remove a placement preference")
	flags.SetAnnotation(flagPlacementPrefRemove, "version", []string{"1.28"})
	flags.Var(&options.networks, flagNetworkAdd, "Add a network")
	flags.SetAnnotation(flagNetworkAdd, "version", []string{"1.29"})
	flags.Var(newListOptsVar(), flagNetworkRemove, "Remove a network")
	flags.SetAnnotation(flagNetworkRemove, "version", []string{"1.29"})
	flags.Var(&options.endpoint.publishPorts, flagPublishAdd, "Add or update a published port")
	flags.Var(&options.groups, flagGroupAdd, "Add an additional supplementary user group to the container")
	flags.SetAnnotation(flagGroupAdd, "version", []string{"1.25"})
	flags.Var(&options.dns, flagDNSAdd, "Add or update a custom DNS server")
	flags.SetAnnotation(flagDNSAdd, "version", []string{"1.25"})
	flags.Var(&options.dnsOption, flagDNSOptionAdd, "Add or update a DNS option")
	flags.SetAnnotation(flagDNSOptionAdd, "version", []string{"1.25"})
	flags.Var(&options.dnsSearch, flagDNSSearchAdd, "Add or update a custom DNS search domain")
	flags.SetAnnotation(flagDNSSearchAdd, "version", []string{"1.25"})
	flags.Var(&options.hosts, flagHostAdd, `Add a custom host-to-IP mapping ("host:ip")`)
	flags.SetAnnotation(flagHostAdd, "version", []string{"1.25"})
	flags.BoolVar(&options.init, flagInit, false, "Use an init inside each service container to forward signals and reap processes")
	flags.SetAnnotation(flagInit, "version", []string{"1.37"})
	flags.Var(&options.sysctls, flagSysCtlAdd, "Add or update a Sysctl option")
	flags.SetAnnotation(flagSysCtlAdd, "version", []string{"1.40"})
	flags.Var(newListOptsVar(), flagSysCtlRemove, "Remove a Sysctl option")
	flags.SetAnnotation(flagSysCtlRemove, "version", []string{"1.40"})
	flags.Var(&options.ulimits, flagUlimitAdd, "Add or update a ulimit option")
	flags.SetAnnotation(flagUlimitAdd, "version", []string{"1.41"})
	flags.Var(newListOptsVar(), flagUlimitRemove, "Remove a ulimit option")
	flags.SetAnnotation(flagUlimitRemove, "version", []string{"1.41"})
	flags.Int64Var(&options.oomScoreAdj, flagOomScoreAdj, 0, "Tune host's OOM preferences (-1000 to 1000) ")
	flags.SetAnnotation(flagOomScoreAdj, "version", []string{"1.46"})

	// Add needs parsing, Remove only needs the key
	flags.Var(newListOptsVar(), flagGenericResourcesRemove, "Remove a Generic resource")
	flags.SetAnnotation(flagHostAdd, "version", []string{"1.32"})
	flags.Var(newListOptsVarWithValidator(ValidateSingleGenericResource), flagGenericResourcesAdd, "Add a Generic resource")
	flags.SetAnnotation(flagHostAdd, "version", []string{"1.32"})

	// TODO(thaJeztah): add completion for capabilities, stop-signal (currently non-exported in container package)
	// _ = cmd.RegisterFlagCompletionFunc(flagCapAdd, completeLinuxCapabilityNames)
	// _ = cmd.RegisterFlagCompletionFunc(flagCapDrop, completeLinuxCapabilityNames)
	// _ = cmd.RegisterFlagCompletionFunc(flagStopSignal, completeSignals)

	_ = cmd.RegisterFlagCompletionFunc(flagEnvAdd, completion.EnvVarNames)
	// TODO(thaJeztah): flagEnvRemove (needs to read current env-vars on the service)
	_ = cmd.RegisterFlagCompletionFunc("image", completion.ImageNames(dockerCLI, -1))
	_ = cmd.RegisterFlagCompletionFunc(flagNetworkAdd, completion.NetworkNames(dockerCLI))
	// TODO(thaJeztha): flagNetworkRemove (needs to read current list of networks from the service)
	_ = cmd.RegisterFlagCompletionFunc(flagRestartCondition, completion.FromList("none", "on-failure", "any"))
	_ = cmd.RegisterFlagCompletionFunc(flagRollbackOrder, completion.FromList("start-first", "stop-first"))
	_ = cmd.RegisterFlagCompletionFunc(flagRollbackFailureAction, completion.FromList("pause", "continue"))
	_ = cmd.RegisterFlagCompletionFunc(flagUpdateOrder, completion.FromList("start-first", "stop-first"))
	_ = cmd.RegisterFlagCompletionFunc(flagUpdateFailureAction, completion.FromList("pause", "continue", "rollback"))

	completion.ImageNames(dockerCLI, -1)
	flags.VisitAll(func(flag *pflag.Flag) {
		// Set a default completion function if none was set. We don't look
		// up if it does already have one set, because Cobra does this for
		// us, and returns an error (which we ignore for this reason).
		_ = cmd.RegisterFlagCompletionFunc(flag.Name, completion.NoComplete)
	})

	return cmd
}

func newListOptsVar() *opts.ListOpts {
	return opts.NewListOptsRef(&[]string{}, nil)
}

func newListOptsVarWithValidator(validator opts.ValidatorFctType) *opts.ListOpts {
	return opts.NewListOptsRef(&[]string{}, validator)
}

//nolint:gocyclo
func runUpdate(ctx context.Context, dockerCLI command.Cli, flags *pflag.FlagSet, options *serviceOptions, serviceID string) error {
	apiClient := dockerCLI.Client()

	service, _, err := apiClient.ServiceInspectWithRaw(ctx, serviceID, swarm.ServiceInspectOptions{})
	if err != nil {
		return err
	}

	rollback, err := flags.GetBool(flagRollback)
	if err != nil {
		return err
	}

	// There are two ways to do user-requested rollback. The old way is
	// client-side, but with a sufficiently recent daemon we prefer
	// server-side, because it will honor the rollback parameters.
	var (
		clientSideRollback bool
		serverSideRollback bool
	)

	spec := &service.Spec
	if rollback {
		// Rollback can't be combined with other flags.
		otherFlagsPassed := false
		flags.VisitAll(func(f *pflag.Flag) {
			if f.Name == flagRollback || f.Name == flagDetach || f.Name == flagQuiet {
				return
			}
			if flags.Changed(f.Name) {
				otherFlagsPassed = true
			}
		})
		if otherFlagsPassed {
			return errors.New("other flags may not be combined with --rollback")
		}

		if versions.LessThan(apiClient.ClientVersion(), "1.28") {
			clientSideRollback = true
			spec = service.PreviousSpec
			if spec == nil {
				return errors.Errorf("service does not have a previous specification to roll back to")
			}
		} else {
			serverSideRollback = true
		}
	}

	updateOpts := swarm.ServiceUpdateOptions{}
	if serverSideRollback {
		updateOpts.Rollback = "previous"
	}

	err = updateService(ctx, apiClient, flags, spec)
	if err != nil {
		return err
	}

	if flags.Changed("image") {
		if err := resolveServiceImageDigestContentTrust(dockerCLI, spec); err != nil {
			return err
		}
		if !options.noResolveImage && versions.GreaterThanOrEqualTo(apiClient.ClientVersion(), "1.30") {
			updateOpts.QueryRegistry = true
		}
	}

	updatedSecrets, err := getUpdatedSecrets(ctx, apiClient, flags, spec.TaskTemplate.ContainerSpec.Secrets)
	if err != nil {
		return err
	}

	spec.TaskTemplate.ContainerSpec.Secrets = updatedSecrets

	updatedConfigs, err := getUpdatedConfigs(ctx, apiClient, flags, spec.TaskTemplate.ContainerSpec)
	if err != nil {
		return err
	}

	spec.TaskTemplate.ContainerSpec.Configs = updatedConfigs

	// set the credential spec value after get the updated configs, because we
	// might need the updated configs to set the correct value of the
	// CredentialSpec.
	updateCredSpecConfig(flags, spec.TaskTemplate.ContainerSpec)

	// only send auth if flag was set
	sendAuth, err := flags.GetBool(flagRegistryAuth)
	if err != nil {
		return err
	}
	switch {
	case sendAuth:
		// Retrieve encoded auth token from the image reference
		// This would be the old image if it didn't change in this update
		image := spec.TaskTemplate.ContainerSpec.Image
		encodedAuth, err := command.RetrieveAuthTokenFromImage(dockerCLI.ConfigFile(), image)
		if err != nil {
			return err
		}
		updateOpts.EncodedRegistryAuth = encodedAuth
	case clientSideRollback:
		updateOpts.RegistryAuthFrom = swarm.RegistryAuthFromPreviousSpec
	default:
		updateOpts.RegistryAuthFrom = swarm.RegistryAuthFromSpec
	}

	response, err := apiClient.ServiceUpdate(ctx, service.ID, service.Version, *spec, updateOpts)
	if err != nil {
		return err
	}

	for _, warning := range response.Warnings {
		_, _ = fmt.Fprintln(dockerCLI.Err(), warning)
	}

	_, _ = fmt.Fprintln(dockerCLI.Out(), serviceID)

	if options.detach || versions.LessThan(apiClient.ClientVersion(), "1.29") {
		return nil
	}

	return WaitOnService(ctx, dockerCLI, serviceID, options.quiet)
}

//nolint:gocyclo
func updateService(ctx context.Context, apiClient client.NetworkAPIClient, flags *pflag.FlagSet, spec *swarm.ServiceSpec) error {
	updateBoolPtr := func(flag string, field **bool) {
		if flags.Changed(flag) {
			b, _ := flags.GetBool(flag)
			*field = &b
		}
	}
	updateString := func(flag string, field *string) {
		if flags.Changed(flag) {
			*field, _ = flags.GetString(flag)
		}
	}

	updateInt64Value := func(flag string, field *int64) {
		if flags.Changed(flag) {
			*field = flags.Lookup(flag).Value.(int64Value).Value()
		}
	}

	updateFloatValue := func(flag string, field *float32) {
		if flags.Changed(flag) {
			*field = flags.Lookup(flag).Value.(*floatValue).Value()
		}
	}

	updateDuration := func(flag string, field *time.Duration) {
		if flags.Changed(flag) {
			*field, _ = flags.GetDuration(flag)
		}
	}

	updateDurationOpt := func(flag string, field **time.Duration) {
		if flags.Changed(flag) {
			val := *flags.Lookup(flag).Value.(*opts.DurationOpt).Value()
			*field = &val
		}
	}

	updateInt64 := func(flag string, field *int64) {
		if flags.Changed(flag) {
			*field, _ = flags.GetInt64(flag)
		}
	}

	updateUint64 := func(flag string, field *uint64) {
		if flags.Changed(flag) {
			*field, _ = flags.GetUint64(flag)
		}
	}

	updateUint64Opt := func(flag string, field **uint64) {
		if flags.Changed(flag) {
			val := *flags.Lookup(flag).Value.(*Uint64Opt).Value()
			*field = &val
		}
	}

	updateIsolation := func(flag string, field *container.Isolation) error {
		if flags.Changed(flag) {
			val, _ := flags.GetString(flag)
			*field = container.Isolation(val)
		}
		return nil
	}

	cspec := spec.TaskTemplate.ContainerSpec
	task := &spec.TaskTemplate

	taskResources := func() *swarm.ResourceRequirements {
		if task.Resources == nil {
			task.Resources = &swarm.ResourceRequirements{}
		}
		if task.Resources.Limits == nil {
			task.Resources.Limits = &swarm.Limit{}
		}
		if task.Resources.Reservations == nil {
			task.Resources.Reservations = &swarm.Resources{}
		}
		return task.Resources
	}

	updateLabels(flags, &spec.Labels)
	updateContainerLabels(flags, &cspec.Labels)
	updateString("image", &cspec.Image)
	updateStringToSlice(flags, "args", &cspec.Args)
	updateStringToSlice(flags, flagEntrypoint, &cspec.Command)
	updateEnvironment(flags, &cspec.Env)
	updateString(flagWorkdir, &cspec.Dir)
	updateString(flagUser, &cspec.User)
	updateString(flagHostname, &cspec.Hostname)
	updateBoolPtr(flagInit, &cspec.Init)
	if err := updateIsolation(flagIsolation, &cspec.Isolation); err != nil {
		return err
	}
	if err := updateMounts(flags, &cspec.Mounts); err != nil {
		return err
	}

	updateSysCtls(flags, &task.ContainerSpec.Sysctls)
	task.ContainerSpec.Ulimits = updateUlimits(flags, task.ContainerSpec.Ulimits)

	if anyChanged(flags, flagLimitCPU, flagLimitMemory, flagLimitPids) {
		taskResources().Limits = spec.TaskTemplate.Resources.Limits
		updateInt64Value(flagLimitCPU, &task.Resources.Limits.NanoCPUs)
		updateInt64Value(flagLimitMemory, &task.Resources.Limits.MemoryBytes)
		updateInt64(flagLimitPids, &task.Resources.Limits.Pids)
	}

	if anyChanged(flags, flagReserveCPU, flagReserveMemory) {
		taskResources().Reservations = spec.TaskTemplate.Resources.Reservations
		updateInt64Value(flagReserveCPU, &task.Resources.Reservations.NanoCPUs)
		updateInt64Value(flagReserveMemory, &task.Resources.Reservations.MemoryBytes)
	}

	if anyChanged(flags, flagOomScoreAdj) {
		updateInt64(flagOomScoreAdj, &task.ContainerSpec.OomScoreAdj)
	}

	if err := addGenericResources(flags, task); err != nil {
		return err
	}

	if err := removeGenericResources(flags, task); err != nil {
		return err
	}

	updateDurationOpt(flagStopGracePeriod, &cspec.StopGracePeriod)

	if anyChanged(flags, flagRestartCondition, flagRestartDelay, flagRestartMaxAttempts, flagRestartWindow) {
		if task.RestartPolicy == nil {
			task.RestartPolicy = defaultRestartPolicy()
		}
		if flags.Changed(flagRestartCondition) {
			value, _ := flags.GetString(flagRestartCondition)
			task.RestartPolicy.Condition = swarm.RestartPolicyCondition(value)
		}
		updateDurationOpt(flagRestartDelay, &task.RestartPolicy.Delay)
		updateUint64Opt(flagRestartMaxAttempts, &task.RestartPolicy.MaxAttempts)
		updateDurationOpt(flagRestartWindow, &task.RestartPolicy.Window)
	}

	if anyChanged(flags, flagConstraintAdd, flagConstraintRemove) {
		if task.Placement == nil {
			task.Placement = &swarm.Placement{}
		}
		updatePlacementConstraints(flags, task.Placement)
	}

	if anyChanged(flags, flagPlacementPrefAdd, flagPlacementPrefRemove) {
		if task.Placement == nil {
			task.Placement = &swarm.Placement{}
		}
		updatePlacementPreferences(flags, task.Placement)
	}

	if anyChanged(flags, flagNetworkAdd, flagNetworkRemove) {
		if err := updateNetworks(ctx, apiClient, flags, spec); err != nil {
			return err
		}
	}

	if err := updateReplicas(flags, &spec.Mode); err != nil {
		return err
	}

	if anyChanged(flags, flagMaxReplicas) {
		updateUint64(flagMaxReplicas, &task.Placement.MaxReplicas)
	}

	if anyChanged(flags, flagUpdateParallelism, flagUpdateDelay, flagUpdateMonitor, flagUpdateFailureAction, flagUpdateMaxFailureRatio, flagUpdateOrder) {
		if spec.UpdateConfig == nil {
			spec.UpdateConfig = updateConfigFromDefaults(defaults.Service.Update)
		}
		updateUint64(flagUpdateParallelism, &spec.UpdateConfig.Parallelism)
		updateDuration(flagUpdateDelay, &spec.UpdateConfig.Delay)
		updateDuration(flagUpdateMonitor, &spec.UpdateConfig.Monitor)
		updateString(flagUpdateFailureAction, &spec.UpdateConfig.FailureAction)
		updateFloatValue(flagUpdateMaxFailureRatio, &spec.UpdateConfig.MaxFailureRatio)
		updateString(flagUpdateOrder, &spec.UpdateConfig.Order)
	}

	if anyChanged(flags, flagRollbackParallelism, flagRollbackDelay, flagRollbackMonitor, flagRollbackFailureAction, flagRollbackMaxFailureRatio, flagRollbackOrder) {
		if spec.RollbackConfig == nil {
			spec.RollbackConfig = updateConfigFromDefaults(defaults.Service.Rollback)
		}
		updateUint64(flagRollbackParallelism, &spec.RollbackConfig.Parallelism)
		updateDuration(flagRollbackDelay, &spec.RollbackConfig.Delay)
		updateDuration(flagRollbackMonitor, &spec.RollbackConfig.Monitor)
		updateString(flagRollbackFailureAction, &spec.RollbackConfig.FailureAction)
		updateFloatValue(flagRollbackMaxFailureRatio, &spec.RollbackConfig.MaxFailureRatio)
		updateString(flagRollbackOrder, &spec.RollbackConfig.Order)
	}

	if flags.Changed(flagEndpointMode) {
		value, _ := flags.GetString(flagEndpointMode)
		if spec.EndpointSpec == nil {
			spec.EndpointSpec = &swarm.EndpointSpec{}
		}
		spec.EndpointSpec.Mode = swarm.ResolutionMode(value)
	}

	if anyChanged(flags, flagGroupAdd, flagGroupRemove) {
		if err := updateGroups(flags, &cspec.Groups); err != nil {
			return err
		}
	}

	if anyChanged(flags, flagPublishAdd, flagPublishRemove) {
		if spec.EndpointSpec == nil {
			spec.EndpointSpec = &swarm.EndpointSpec{}
		}
		if err := updatePorts(flags, &spec.EndpointSpec.Ports); err != nil {
			return err
		}
	}

	if anyChanged(flags, flagDNSAdd, flagDNSRemove, flagDNSOptionAdd, flagDNSOptionRemove, flagDNSSearchAdd, flagDNSSearchRemove) {
		if cspec.DNSConfig == nil {
			cspec.DNSConfig = &swarm.DNSConfig{}
		}
		if err := updateDNSConfig(flags, &cspec.DNSConfig); err != nil {
			return err
		}
	}

	if anyChanged(flags, flagHostAdd, flagHostRemove) {
		if err := updateHosts(flags, &cspec.Hosts); err != nil {
			return err
		}
	}

	if err := updateLogDriver(flags, &spec.TaskTemplate); err != nil {
		return err
	}

	force, err := flags.GetBool("force")
	if err != nil {
		return err
	}

	if force {
		spec.TaskTemplate.ForceUpdate++
	}

	if err := updateHealthcheck(flags, cspec); err != nil {
		return err
	}

	if flags.Changed(flagTTY) {
		tty, err := flags.GetBool(flagTTY)
		if err != nil {
			return err
		}
		cspec.TTY = tty
	}

	if flags.Changed(flagReadOnly) {
		readOnly, err := flags.GetBool(flagReadOnly)
		if err != nil {
			return err
		}
		cspec.ReadOnly = readOnly
	}

	updateString(flagStopSignal, &cspec.StopSignal)

	if anyChanged(flags, flagCapAdd, flagCapDrop) {
		updateCapabilities(flags, cspec)
	}

	return nil
}

func updateStringToSlice(flags *pflag.FlagSet, flag string, field *[]string) {
	if !flags.Changed(flag) {
		return
	}

	*field = flags.Lookup(flag).Value.(*ShlexOpt).Value()
}

func anyChanged(flags *pflag.FlagSet, fields ...string) bool {
	for _, flag := range fields {
		if flags.Changed(flag) {
			return true
		}
	}
	return false
}

func addGenericResources(flags *pflag.FlagSet, spec *swarm.TaskSpec) error {
	if !flags.Changed(flagGenericResourcesAdd) {
		return nil
	}

	if spec.Resources == nil {
		spec.Resources = &swarm.ResourceRequirements{}
	}

	if spec.Resources.Reservations == nil {
		spec.Resources.Reservations = &swarm.Resources{}
	}

	values := flags.Lookup(flagGenericResourcesAdd).Value.(*opts.ListOpts).GetSlice()
	generic, err := ParseGenericResources(values)
	if err != nil {
		return err
	}

	m, err := buildGenericResourceMap(spec.Resources.Reservations.GenericResources)
	if err != nil {
		return err
	}

	for _, toAddRes := range generic {
		m[toAddRes.DiscreteResourceSpec.Kind] = toAddRes
	}

	spec.Resources.Reservations.GenericResources = buildGenericResourceList(m)

	return nil
}

func removeGenericResources(flags *pflag.FlagSet, spec *swarm.TaskSpec) error {
	// Can only be Discrete Resources
	if !flags.Changed(flagGenericResourcesRemove) {
		return nil
	}

	if spec.Resources == nil {
		spec.Resources = &swarm.ResourceRequirements{}
	}

	if spec.Resources.Reservations == nil {
		spec.Resources.Reservations = &swarm.Resources{}
	}

	values := flags.Lookup(flagGenericResourcesRemove).Value.(*opts.ListOpts).GetSlice()

	m, err := buildGenericResourceMap(spec.Resources.Reservations.GenericResources)
	if err != nil {
		return err
	}

	for _, toRemoveRes := range values {
		if _, ok := m[toRemoveRes]; !ok {
			return fmt.Errorf("could not find generic-resource `%s` to remove it", toRemoveRes)
		}

		delete(m, toRemoveRes)
	}

	spec.Resources.Reservations.GenericResources = buildGenericResourceList(m)
	return nil
}

func updatePlacementConstraints(flags *pflag.FlagSet, placement *swarm.Placement) {
	if flags.Changed(flagConstraintAdd) {
		values := flags.Lookup(flagConstraintAdd).Value.(*opts.ListOpts).GetSlice()
		placement.Constraints = append(placement.Constraints, values...)
	}
	toRemove := buildToRemoveSet(flags, flagConstraintRemove)

	newConstraints := []string{}
	for _, constraint := range placement.Constraints {
		if _, exists := toRemove[constraint]; !exists {
			newConstraints = append(newConstraints, constraint)
		}
	}
	// Sort so that result is predictable.
	sort.Strings(newConstraints)

	placement.Constraints = newConstraints
}

func updatePlacementPreferences(flags *pflag.FlagSet, placement *swarm.Placement) {
	var newPrefs []swarm.PlacementPreference

	if flags.Changed(flagPlacementPrefRemove) {
		for _, existing := range placement.Preferences {
			removed := false
			for _, removal := range flags.Lookup(flagPlacementPrefRemove).Value.(*placementPrefOpts).prefs {
				if removal.Spread != nil && existing.Spread != nil && removal.Spread.SpreadDescriptor == existing.Spread.SpreadDescriptor {
					removed = true
					break
				}
			}
			if !removed {
				newPrefs = append(newPrefs, existing)
			}
		}
	} else {
		newPrefs = placement.Preferences
	}

	if flags.Changed(flagPlacementPrefAdd) {
		newPrefs = append(newPrefs,
			flags.Lookup(flagPlacementPrefAdd).Value.(*placementPrefOpts).prefs...)
	}

	placement.Preferences = newPrefs
}

func updateContainerLabels(flags *pflag.FlagSet, field *map[string]string) {
	if *field != nil && flags.Changed(flagContainerLabelRemove) {
		toRemove := flags.Lookup(flagContainerLabelRemove).Value.(*opts.ListOpts).GetSlice()
		for _, label := range toRemove {
			delete(*field, label)
		}
	}
	if flags.Changed(flagContainerLabelAdd) {
		if *field == nil {
			*field = map[string]string{}
		}

		values := flags.Lookup(flagContainerLabelAdd).Value.(*opts.ListOpts).GetSlice()
		for key, value := range opts.ConvertKVStringsToMap(values) {
			(*field)[key] = value
		}
	}
}

func updateLabels(flags *pflag.FlagSet, field *map[string]string) {
	if *field != nil && flags.Changed(flagLabelRemove) {
		toRemove := flags.Lookup(flagLabelRemove).Value.(*opts.ListOpts).GetSlice()
		for _, label := range toRemove {
			delete(*field, label)
		}
	}
	if flags.Changed(flagLabelAdd) {
		if *field == nil {
			*field = map[string]string{}
		}

		values := flags.Lookup(flagLabelAdd).Value.(*opts.ListOpts).GetSlice()
		for key, value := range opts.ConvertKVStringsToMap(values) {
			(*field)[key] = value
		}
	}
}

func updateSysCtls(flags *pflag.FlagSet, field *map[string]string) {
	if *field != nil && flags.Changed(flagSysCtlRemove) {
		values := flags.Lookup(flagSysCtlRemove).Value.(*opts.ListOpts).GetSlice()
		for key := range opts.ConvertKVStringsToMap(values) {
			delete(*field, key)
		}
	}
	if flags.Changed(flagSysCtlAdd) {
		if *field == nil {
			*field = map[string]string{}
		}

		values := flags.Lookup(flagSysCtlAdd).Value.(*opts.ListOpts).GetSlice()
		for key, value := range opts.ConvertKVStringsToMap(values) {
			(*field)[key] = value
		}
	}
}

func updateUlimits(flags *pflag.FlagSet, ulimits []*container.Ulimit) []*container.Ulimit {
	newUlimits := make(map[string]*container.Ulimit)

	for _, ulimit := range ulimits {
		newUlimits[ulimit.Name] = ulimit
	}
	if flags.Changed(flagUlimitRemove) {
		values := flags.Lookup(flagUlimitRemove).Value.(*opts.ListOpts).GetSlice()
		for key := range opts.ConvertKVStringsToMap(values) {
			delete(newUlimits, key)
		}
	}

	if flags.Changed(flagUlimitAdd) {
		for _, ulimit := range flags.Lookup(flagUlimitAdd).Value.(*opts.UlimitOpt).GetList() {
			newUlimits[ulimit.Name] = ulimit
		}
	}
	if len(newUlimits) == 0 {
		return nil
	}
	limits := make([]*container.Ulimit, 0, len(newUlimits))
	for _, ulimit := range newUlimits {
		limits = append(limits, ulimit)
	}
	sort.SliceStable(limits, func(i, j int) bool {
		return limits[i].Name < limits[j].Name
	})
	return limits
}

func updateEnvironment(flags *pflag.FlagSet, field *[]string) {
	toRemove := buildToRemoveSet(flags, flagEnvRemove)
	*field = removeItems(*field, toRemove, envKey)

	if flags.Changed(flagEnvAdd) {
		envSet := map[string]string{}
		for _, v := range *field {
			envSet[envKey(v)] = v
		}

		value := flags.Lookup(flagEnvAdd).Value.(*opts.ListOpts)
		for _, v := range value.GetSlice() {
			envSet[envKey(v)] = v
		}

		*field = []string{}
		for _, v := range envSet {
			*field = append(*field, v)
		}
	}
}

func getUpdatedSecrets(ctx context.Context, apiClient client.SecretAPIClient, flags *pflag.FlagSet, secrets []*swarm.SecretReference) ([]*swarm.SecretReference, error) {
	newSecrets := []*swarm.SecretReference{}

	toRemove := buildToRemoveSet(flags, flagSecretRemove)
	for _, secret := range secrets {
		if _, exists := toRemove[secret.SecretName]; !exists {
			newSecrets = append(newSecrets, secret)
		}
	}

	if flags.Changed(flagSecretAdd) {
		values := flags.Lookup(flagSecretAdd).Value.(*swarmopts.SecretOpt).Value()

		addSecrets, err := ParseSecrets(ctx, apiClient, values)
		if err != nil {
			return nil, err
		}
		newSecrets = append(newSecrets, addSecrets...)
	}

	return newSecrets, nil
}

func getUpdatedConfigs(ctx context.Context, apiClient client.ConfigAPIClient, flags *pflag.FlagSet, spec *swarm.ContainerSpec) ([]*swarm.ConfigReference, error) {
	var (
		// credSpecConfigName stores the name of the config specified by the
		// credential-spec flag. if a Runtime target Config with this name is
		// already in the containerSpec, then this value will be set to
		// emptystring in the removeConfigs stage. otherwise, a ConfigReference
		// will be created to pass to ParseConfigs to get the ConfigID.
		credSpecConfigName string
		// credSpecConfigID stores the ID of the credential spec config if that
		// config is being carried over from the old set of references
		credSpecConfigID string
	)

	if flags.Changed(flagCredentialSpec) {
		credSpec := flags.Lookup(flagCredentialSpec).Value.(*credentialSpecOpt).Value()
		credSpecConfigName = credSpec.Config
	} else { //nolint:gocritic // ignore  elseif: can replace 'else {if cond {}}' with 'else if cond {}'
		// if the credential spec flag has not changed, then check if there
		// already is a credentialSpec. if there is one, and it's for a Config,
		// then it's from the old object, and its value is the config ID. we
		// need this so we don't remove the config if the credential spec is
		// not being updated.
		if spec.Privileges != nil && spec.Privileges.CredentialSpec != nil {
			if config := spec.Privileges.CredentialSpec.Config; config != "" {
				credSpecConfigID = config
			}
		}
	}

	newConfigs := removeConfigs(flags, spec, credSpecConfigName, credSpecConfigID)

	// resolveConfigs is a slice of any new configs that need to have the ID
	// resolved
	resolveConfigs := []*swarm.ConfigReference{}

	if flags.Changed(flagConfigAdd) {
		resolveConfigs = append(resolveConfigs, flags.Lookup(flagConfigAdd).Value.(*swarmopts.ConfigOpt).Value()...)
	}

	// if credSpecConfigNameis non-empty at this point, it means its a new
	// config, and we need to resolve its ID accordingly.
	if credSpecConfigName != "" {
		resolveConfigs = append(resolveConfigs, &swarm.ConfigReference{
			ConfigName: credSpecConfigName,
			Runtime:    &swarm.ConfigReferenceRuntimeTarget{},
		})
	}

	if len(resolveConfigs) > 0 {
		addConfigs, err := ParseConfigs(ctx, apiClient, resolveConfigs)
		if err != nil {
			return nil, err
		}
		newConfigs = append(newConfigs, addConfigs...)
	}

	return newConfigs, nil
}

// removeConfigs figures out which configs in the existing spec should be kept
// after the update.
func removeConfigs(flags *pflag.FlagSet, spec *swarm.ContainerSpec, credSpecName, credSpecID string) []*swarm.ConfigReference {
	keepConfigs := []*swarm.ConfigReference{}

	toRemove := buildToRemoveSet(flags, flagConfigRemove)
	// all configs in spec.Configs should have both a Name and ID, because
	// they come from an already-accepted spec.
	for _, config := range spec.Configs {
		// if the config is a Runtime target, make sure it's still in use right
		// now, the only use for Runtime target is credential specs.  if, in
		// the future, more uses are added, then this check will need to be
		// made more intelligent.
		if config.Runtime != nil {
			// if we're carrying over a credential spec explicitly (because the
			// user passed --credential-spec with the same config name) then we
			// should match on credSpecName. if we're carrying over a
			// credential spec implicitly (because the user did not pass any
			// --credential-spec flag) then we should match on credSpecID. in
			// either case, we're keeping the config that already exists.
			if config.ConfigName == credSpecName || config.ConfigID == credSpecID {
				keepConfigs = append(keepConfigs, config)
			}
			// continue the loop, to skip the part where we check if the config
			// is in toRemove.
			continue
		}

		if _, exists := toRemove[config.ConfigName]; !exists {
			keepConfigs = append(keepConfigs, config)
		}
	}

	return keepConfigs
}

func envKey(value string) string {
	k, _, _ := strings.Cut(value, "=")
	return k
}

func buildToRemoveSet(flags *pflag.FlagSet, flag string) map[string]struct{} {
	var empty struct{}
	toRemove := make(map[string]struct{})

	if !flags.Changed(flag) {
		return toRemove
	}

	toRemoveSlice := flags.Lookup(flag).Value.(*opts.ListOpts).GetSlice()
	for _, key := range toRemoveSlice {
		toRemove[key] = empty
	}
	return toRemove
}

func removeItems(
	seq []string,
	toRemove map[string]struct{},
	keyFunc func(string) string,
) []string {
	newSeq := []string{}
	for _, item := range seq {
		if _, exists := toRemove[keyFunc(item)]; !exists {
			newSeq = append(newSeq, item)
		}
	}
	return newSeq
}

func updateMounts(flags *pflag.FlagSet, mounts *[]mount.Mount) error {
	mountsByTarget := map[string]mount.Mount{}

	if flags.Changed(flagMountAdd) {
		values := flags.Lookup(flagMountAdd).Value.(*opts.MountOpt).Value()
		for _, mnt := range values {
			if _, ok := mountsByTarget[mnt.Target]; ok {
				return errors.Errorf("duplicate mount target")
			}
			mountsByTarget[mnt.Target] = mnt
		}
	}

	// Add old list of mount points minus updated one.
	for _, mnt := range *mounts {
		if _, ok := mountsByTarget[mnt.Target]; !ok {
			mountsByTarget[mnt.Target] = mnt
		}
	}

	newMounts := make([]mount.Mount, 0, len(mountsByTarget))

	toRemove := buildToRemoveSet(flags, flagMountRemove)

	for _, mnt := range mountsByTarget {
		if _, exists := toRemove[mnt.Target]; !exists {
			newMounts = append(newMounts, mnt)
		}
	}
	sort.Slice(newMounts, func(i, j int) bool {
		a, b := newMounts[i], newMounts[j]

		if a.Source == b.Source {
			return a.Target < b.Target
		}

		return a.Source < b.Source
	})
	*mounts = newMounts
	return nil
}

func updateGroups(flags *pflag.FlagSet, groups *[]string) error {
	if flags.Changed(flagGroupAdd) {
		values := flags.Lookup(flagGroupAdd).Value.(*opts.ListOpts).GetSlice()
		*groups = append(*groups, values...)
	}
	toRemove := buildToRemoveSet(flags, flagGroupRemove)

	newGroups := []string{}
	for _, group := range *groups {
		if _, exists := toRemove[group]; !exists {
			newGroups = append(newGroups, group)
		}
	}
	// Sort so that result is predictable.
	sort.Strings(newGroups)

	*groups = newGroups
	return nil
}

func removeDuplicates(entries []string) []string {
	hit := map[string]bool{}
	newEntries := []string{}
	for _, v := range entries {
		if !hit[v] {
			newEntries = append(newEntries, v)
			hit[v] = true
		}
	}
	return newEntries
}

func updateDNSConfig(flags *pflag.FlagSet, config **swarm.DNSConfig) error {
	newConfig := &swarm.DNSConfig{}

	nameservers := (*config).Nameservers
	if flags.Changed(flagDNSAdd) {
		values := flags.Lookup(flagDNSAdd).Value.(*opts.ListOpts).GetSlice()
		nameservers = append(nameservers, values...)
	}
	nameservers = removeDuplicates(nameservers)
	toRemove := buildToRemoveSet(flags, flagDNSRemove)
	for _, nameserver := range nameservers {
		if _, exists := toRemove[nameserver]; !exists {
			newConfig.Nameservers = append(newConfig.Nameservers, nameserver)
		}
	}
	// Sort so that result is predictable.
	sort.Strings(newConfig.Nameservers)

	search := (*config).Search
	if flags.Changed(flagDNSSearchAdd) {
		values := flags.Lookup(flagDNSSearchAdd).Value.(*opts.ListOpts).GetSlice()
		search = append(search, values...)
	}
	search = removeDuplicates(search)
	toRemove = buildToRemoveSet(flags, flagDNSSearchRemove)
	for _, entry := range search {
		if _, exists := toRemove[entry]; !exists {
			newConfig.Search = append(newConfig.Search, entry)
		}
	}
	// Sort so that result is predictable.
	sort.Strings(newConfig.Search)

	options := (*config).Options
	if flags.Changed(flagDNSOptionAdd) {
		values := flags.Lookup(flagDNSOptionAdd).Value.(*opts.ListOpts).GetSlice()
		options = append(options, values...)
	}
	options = removeDuplicates(options)
	toRemove = buildToRemoveSet(flags, flagDNSOptionRemove)
	for _, option := range options {
		if _, exists := toRemove[option]; !exists {
			newConfig.Options = append(newConfig.Options, option)
		}
	}
	// Sort so that result is predictable.
	sort.Strings(newConfig.Options)

	*config = newConfig
	return nil
}

func portConfigToString(portConfig *swarm.PortConfig) string {
	protocol := portConfig.Protocol
	mode := portConfig.PublishMode
	return fmt.Sprintf("%v:%v/%s/%s", portConfig.PublishedPort, portConfig.TargetPort, protocol, mode)
}

func updatePorts(flags *pflag.FlagSet, portConfig *[]swarm.PortConfig) error {
	// The key of the map is `port/protocol`, e.g., `80/tcp`
	portSet := map[string]swarm.PortConfig{}

	// Build the current list of portConfig
	for _, entry := range *portConfig {
		if _, ok := portSet[portConfigToString(&entry)]; !ok {
			portSet[portConfigToString(&entry)] = entry
		}
	}

	newPorts := []swarm.PortConfig{}

	// Clean current ports
	toRemove := flags.Lookup(flagPublishRemove).Value.(*swarmopts.PortOpt).Value()
portLoop:
	for _, port := range portSet {
		for _, pConfig := range toRemove {
			if equalProtocol(port.Protocol, pConfig.Protocol) &&
				port.TargetPort == pConfig.TargetPort &&
				equalPublishMode(port.PublishMode, pConfig.PublishMode) {
				continue portLoop
			}
		}

		newPorts = append(newPorts, port)
	}

	// Check to see if there are any conflict in flags.
	if flags.Changed(flagPublishAdd) {
		ports := flags.Lookup(flagPublishAdd).Value.(*swarmopts.PortOpt).Value()

		for _, port := range ports {
			if _, ok := portSet[portConfigToString(&port)]; ok {
				continue
			}
			// portSet[portConfigToString(&port)] = port
			newPorts = append(newPorts, port)
		}
	}

	// Sort the PortConfig to avoid unnecessary updates
	sort.Slice(newPorts, func(i, j int) bool {
		// We convert PortConfig into `port/protocol`, e.g., `80/tcp`
		// In updatePorts we already filter out with map so there is duplicate entries
		return portConfigToString(&newPorts[i]) < portConfigToString(&newPorts[j])
	})
	*portConfig = newPorts
	return nil
}

func equalProtocol(prot1, prot2 swarm.PortConfigProtocol) bool {
	return prot1 == prot2 ||
		(prot1 == swarm.PortConfigProtocol("") && prot2 == swarm.PortConfigProtocolTCP) ||
		(prot2 == swarm.PortConfigProtocol("") && prot1 == swarm.PortConfigProtocolTCP)
}

func equalPublishMode(mode1, mode2 swarm.PortConfigPublishMode) bool {
	return mode1 == mode2 ||
		(mode1 == swarm.PortConfigPublishMode("") && mode2 == swarm.PortConfigPublishModeIngress) ||
		(mode2 == swarm.PortConfigPublishMode("") && mode1 == swarm.PortConfigPublishModeIngress)
}

func updateReplicas(flags *pflag.FlagSet, serviceMode *swarm.ServiceMode) error {
	if !flags.Changed(flagReplicas) {
		return nil
	}

	if serviceMode == nil || serviceMode.Replicated == nil {
		return errors.Errorf("replicas can only be used with replicated mode")
	}
	serviceMode.Replicated.Replicas = flags.Lookup(flagReplicas).Value.(*Uint64Opt).Value()
	return nil
}

type hostMapping struct {
	IPAddr string
	Host   string
}

// updateHosts performs a diff between existing host entries, entries to be
// removed, and entries to be added. Host entries preserve the order in which they
// were added, as the specification mentions that in case multiple entries for a
// host exist, the first entry should be used (by default).
//
// Note that, even though unsupported by the CLI, the service specs format
// allow entries with both a _canonical_ hostname, and one or more aliases
// in an entry (IP-address canonical_hostname [alias ...])
//
// Entries can be removed by either a specific `<host-name>:<ip-address>` mapping,
// or by `<host>` alone:
//
//   - If both IP-address and host-name is provided, the hostname is removed only
//     from entries that match the given IP-address.
//   - If only a host-name is provided, the hostname is removed from any entry it
//     is part of (either as canonical host-name, or as alias).
//   - If, after removing the host-name from an entry, no host-names remain in
//     the entry, the entry itself is removed.
//
// For example, the list of host-entries before processing could look like this:
//
//	hosts = &[]string{
//		"127.0.0.2 host3 host1 host2 host4",
//		"127.0.0.1 host1 host4",
//		"127.0.0.3 host1",
//		"127.0.0.1 host1",
//	}
//
// Removing `host1` removes every occurrence:
//
//	hosts = &[]string{
//		"127.0.0.2 host3 host2 host4",
//		"127.0.0.1 host4",
//	}
//
// Removing `host1:127.0.0.1` on the other hand, only remove the host if the
// IP-address matches:
//
//	hosts = &[]string{
//		"127.0.0.2 host3 host1 host2 host4",
//		"127.0.0.1 host4",
//		"127.0.0.3 host1",
//	}
func updateHosts(flags *pflag.FlagSet, hosts *[]string) error {
	var toRemove []hostMapping
	if flags.Changed(flagHostRemove) {
		extraHostsToRemove := flags.Lookup(flagHostRemove).Value.(*opts.ListOpts).GetSlice()
		for _, entry := range extraHostsToRemove {
			hostName, ipAddr, _ := strings.Cut(entry, ":")
			toRemove = append(toRemove, hostMapping{IPAddr: ipAddr, Host: hostName})
		}
	}

	var newHosts []string
	for _, entry := range *hosts {
		// Since this is in SwarmKit format, we need to find the key, which is canonical_hostname of:
		// IP_address canonical_hostname [aliases...]
		parts := strings.Fields(entry)
		if len(parts) == 0 {
			continue
		}
		ip := parts[0]
		hostNames := parts[1:]
		for _, rm := range toRemove {
			if rm.IPAddr != "" && rm.IPAddr != ip {
				continue
			}
			for i, h := range hostNames {
				if h == rm.Host {
					hostNames = append(hostNames[:i], hostNames[i+1:]...)
				}
			}
		}
		if len(hostNames) > 0 {
			newHosts = append(newHosts, fmt.Sprintf("%s %s", ip, strings.Join(hostNames, " ")))
		}
	}

	// Append new hosts (in SwarmKit format)
	if flags.Changed(flagHostAdd) {
		values := convertExtraHostsToSwarmHosts(flags.Lookup(flagHostAdd).Value.(*opts.ListOpts).GetSlice())
		newHosts = append(newHosts, values...)
	}
	*hosts = removeDuplicates(newHosts)
	return nil
}

// updateLogDriver updates the log driver only if the log driver flag is set.
// All options will be replaced with those provided on the command line.
func updateLogDriver(flags *pflag.FlagSet, taskTemplate *swarm.TaskSpec) error {
	if !flags.Changed(flagLogDriver) {
		return nil
	}

	name, err := flags.GetString(flagLogDriver)
	if err != nil {
		return err
	}

	if name == "" {
		return nil
	}

	taskTemplate.LogDriver = &swarm.Driver{
		Name:    name,
		Options: opts.ConvertKVStringsToMap(flags.Lookup(flagLogOpt).Value.(*opts.ListOpts).GetSlice()),
	}

	return nil
}

func updateHealthcheck(flags *pflag.FlagSet, containerSpec *swarm.ContainerSpec) error {
	if !anyChanged(flags, flagNoHealthcheck, flagHealthCmd, flagHealthInterval, flagHealthRetries, flagHealthTimeout, flagHealthStartPeriod) {
		return nil
	}
	if containerSpec.Healthcheck == nil {
		containerSpec.Healthcheck = &container.HealthConfig{}
	}
	noHealthcheck, err := flags.GetBool(flagNoHealthcheck)
	if err != nil {
		return err
	}
	if noHealthcheck {
		if !anyChanged(flags, flagHealthCmd, flagHealthInterval, flagHealthRetries, flagHealthTimeout, flagHealthStartPeriod) {
			containerSpec.Healthcheck = &container.HealthConfig{
				Test: []string{"NONE"},
			}
			return nil
		}
		return errors.Errorf("--%s conflicts with --health-* options", flagNoHealthcheck)
	}
	if len(containerSpec.Healthcheck.Test) > 0 && containerSpec.Healthcheck.Test[0] == "NONE" {
		containerSpec.Healthcheck.Test = nil
	}
	if flags.Changed(flagHealthInterval) {
		val := *flags.Lookup(flagHealthInterval).Value.(*opts.PositiveDurationOpt).Value()
		containerSpec.Healthcheck.Interval = val
	}
	if flags.Changed(flagHealthTimeout) {
		val := *flags.Lookup(flagHealthTimeout).Value.(*opts.PositiveDurationOpt).Value()
		containerSpec.Healthcheck.Timeout = val
	}
	if flags.Changed(flagHealthStartPeriod) {
		val := *flags.Lookup(flagHealthStartPeriod).Value.(*opts.PositiveDurationOpt).Value()
		containerSpec.Healthcheck.StartPeriod = val
	}
	if flags.Changed(flagHealthRetries) {
		containerSpec.Healthcheck.Retries, _ = flags.GetInt(flagHealthRetries)
	}
	if flags.Changed(flagHealthCmd) {
		cmd, _ := flags.GetString(flagHealthCmd)
		if cmd != "" {
			containerSpec.Healthcheck.Test = []string{"CMD-SHELL", cmd}
		} else {
			containerSpec.Healthcheck.Test = nil
		}
	}
	return nil
}

func updateNetworks(ctx context.Context, apiClient client.NetworkAPIClient, flags *pflag.FlagSet, spec *swarm.ServiceSpec) error {
	// spec.TaskTemplate.Networks takes precedence over the deprecated
	// spec.Networks field. If spec.Network is in use, we'll migrate those
	// values to spec.TaskTemplate.Networks.
	specNetworks := spec.TaskTemplate.Networks
	if len(specNetworks) == 0 {
		specNetworks = spec.Networks //nolint:staticcheck // ignore SA1019: field is deprecated.
	}
	spec.Networks = nil //nolint:staticcheck // ignore SA1019: field is deprecated.

	toRemove := buildToRemoveSet(flags, flagNetworkRemove)
	idsToRemove := make(map[string]struct{})
	for networkIDOrName := range toRemove {
		nw, err := apiClient.NetworkInspect(ctx, networkIDOrName, network.InspectOptions{Scope: "swarm"})
		if err != nil {
			return err
		}
		idsToRemove[nw.ID] = struct{}{}
	}

	existingNetworks := make(map[string]struct{})
	var newNetworks []swarm.NetworkAttachmentConfig //nolint:prealloc
	for _, nw := range specNetworks {
		if _, exists := idsToRemove[nw.Target]; exists {
			continue
		}

		newNetworks = append(newNetworks, nw)
		existingNetworks[nw.Target] = struct{}{}
	}

	if flags.Changed(flagNetworkAdd) {
		values := flags.Lookup(flagNetworkAdd).Value.(*opts.NetworkOpt)
		networks := convertNetworks(*values)
		for _, nw := range networks {
			nwID, err := resolveNetworkID(ctx, apiClient, nw.Target)
			if err != nil {
				return err
			}
			if _, exists := existingNetworks[nwID]; exists {
				return errors.Errorf("service is already attached to network %s", nw.Target)
			}
			nw.Target = nwID
			newNetworks = append(newNetworks, nw)
			existingNetworks[nw.Target] = struct{}{}
		}
	}

	sort.Slice(newNetworks, func(i, j int) bool {
		return newNetworks[i].Target < newNetworks[j].Target
	})

	spec.TaskTemplate.Networks = newNetworks
	return nil
}

// updateCredSpecConfig updates the value of the credential spec Config field
// to the config ID if the credential spec has changed. it mutates the passed
// spec. it does not handle the case where the credential spec specifies a
// config that does not exist -- that case is handled as part of
// getUpdatedConfigs
func updateCredSpecConfig(flags *pflag.FlagSet, containerSpec *swarm.ContainerSpec) {
	if flags.Changed(flagCredentialSpec) {
		credSpecOpt := flags.Lookup(flagCredentialSpec)
		// if the flag has changed, and the value is empty string, then we
		// should remove any credential spec that might be present
		if credSpecOpt.Value.String() == "" {
			if containerSpec.Privileges != nil {
				containerSpec.Privileges.CredentialSpec = nil
			}
			return
		}

		// otherwise, set the credential spec to be the parsed value
		credSpec := credSpecOpt.Value.(*credentialSpecOpt).Value()

		// if this is a Config credential spec, we still need to replace the
		// value of credSpec.Config with the config ID instead of Name.
		if credSpec.Config != "" {
			for _, config := range containerSpec.Configs {
				// if the config name matches, then set the config ID. we do
				// not need to worry about if this is a Runtime target or not.
				// even if it is not a Runtime target, getUpdatedConfigs
				// ensures that a Runtime target for this config exists, and
				// the Name is unique so the ID is correct no matter the
				// target.
				if config.ConfigName == credSpec.Config {
					credSpec.Config = config.ConfigID
					break
				}
			}
		}

		if containerSpec.Privileges == nil {
			containerSpec.Privileges = &swarm.Privileges{}
		}

		containerSpec.Privileges.CredentialSpec = credSpec
	}
}

// updateCapabilities calculates the list of capabilities to "drop" and to "add"
// after applying the capabilities passed through `--cap-add` and `--cap-drop`
// to the existing list of added/dropped capabilities in the service spec.
//
// Adding capabilities takes precedence over "dropping" the same capability, so
// if both `--cap-add` and `--cap-drop` are specifying the same capability, the
// `--cap-drop` is ignored.
//
// Capabilities to "drop" are removed from the existing list of "added"
// capabilities, and vice-versa (capabilities to "add" are removed from the existing
// list of capabilities to "drop").
//
// Capabilities are normalized, sorted, and duplicates are removed to prevent
// service tasks from being updated if no changes are made. If a list has the "ALL"
// capability set, then any other capability is removed from that list.
//
// Adding/removing capabilities when updating a service is handled as a tri-state;
//
//   - if the capability was previously "dropped", then remove it from "CapabilityDrop",
//     but NOT added to "CapabilityAdd". However, if the capability was not yet in
//     the service's "CapabilityDrop", then it's simply added to the service's "CapabilityAdd"
//   - likewise, if the capability was previously "added", then it's removed from
//     "CapabilityAdd", but NOT added to "CapabilityDrop". If the capability was
//     not yet in the service's "CapabilityAdd", then simply add it to the service's
//     "CapabilityDrop".
//
// In other words, given a service with the following:
//
// | CapDrop        | CapAdd        |
// |----------------|---------------|
// | CAP_SOME_CAP   |               |
//
// When updating the service, and applying `--cap-add CAP_SOME_CAP`, the previously
// dropped capability is removed:
//
// | CapDrop        | CapAdd        |
// |----------------|---------------|
// |                |               |
//
// After updating the service a second time, applying `--cap-add CAP_SOME_CAP`,
// capability is now added:
//
// | CapDrop        | CapAdd        |
// |----------------|---------------|
// |                | CAP_SOME_CAP  |
func updateCapabilities(flags *pflag.FlagSet, containerSpec *swarm.ContainerSpec) {
	var (
		toAdd, toDrop map[string]bool

		capDrop = opts.CapabilitiesMap(containerSpec.CapabilityDrop)
		capAdd  = opts.CapabilitiesMap(containerSpec.CapabilityAdd)
	)
	if flags.Changed(flagCapAdd) {
		toAdd = opts.CapabilitiesMap(flags.Lookup(flagCapAdd).Value.(*opts.ListOpts).GetSlice())
		if toAdd[opts.ResetCapabilities] {
			capAdd = make(map[string]bool)
			delete(toAdd, opts.ResetCapabilities)
		}
	}
	if flags.Changed(flagCapDrop) {
		toDrop = opts.CapabilitiesMap(flags.Lookup(flagCapDrop).Value.(*opts.ListOpts).GetSlice())
		if toDrop[opts.ResetCapabilities] {
			capDrop = make(map[string]bool)
			delete(toDrop, opts.ResetCapabilities)
		}
	}

	// First remove the capabilities to "drop" from the service's exiting
	// list of capabilities to "add". If a capability is both added and dropped
	// on update, then "adding" takes precedence.
	//
	// Dropping a capability when updating a service is considered a tri-state;
	//
	// - if the capability was previously "added", then remove it from
	//   "CapabilityAdd", and do NOT add it to "CapabilityDrop"
	// - if the capability was not yet in the service's "CapabilityAdd",
	//   then simply add it to the service's "CapabilityDrop"
	for c := range toDrop {
		if !toAdd[c] {
			if capAdd[c] {
				delete(capAdd, c)
			} else {
				capDrop[c] = true
			}
		}
	}

	// And remove the capabilities we're "adding" from the service's existing
	// list of capabilities to "drop".
	//
	// "Adding" capabilities takes precedence over "dropping" them, so if a
	// capability is set both as "add" and "drop", remove the capability from
	// the service's list of dropped capabilities (if present).
	//
	// Adding a capability when updating a service is considered a tri-state;
	//
	// - if the capability was previously "dropped", then remove it from
	//   "CapabilityDrop", and do NOT add it to "CapabilityAdd"
	// - if the capability was not yet in the service's "CapabilityDrop",
	//   then simply add it to the service's "CapabilityAdd"
	for c := range toAdd {
		if capDrop[c] {
			delete(capDrop, c)
		} else {
			capAdd[c] = true
		}
	}

	// Now that the service's existing lists are updated, apply the new
	// capabilities to add/drop to both lists. Sort the lists to prevent
	// unneeded updates to service-tasks.
	containerSpec.CapabilityDrop = capsList(capDrop)
	containerSpec.CapabilityAdd = capsList(capAdd)
}

func capsList(caps map[string]bool) []string {
	if len(caps) == 0 {
		return nil
	}
	if caps[opts.AllCapabilities] {
		return []string{opts.AllCapabilities}
	}
	out := make([]string, 0, len(caps))
	for c := range caps {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}
