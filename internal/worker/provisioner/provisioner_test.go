// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provisioner_test

import (
	stdcontext "context"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/juju/collections/set"
	"github.com/juju/errors"
	"github.com/juju/loggo"
	"github.com/juju/names/v5"
	jc "github.com/juju/testing/checkers"
	"github.com/juju/utils/v3"
	"github.com/juju/version/v2"
	"github.com/juju/worker/v3"
	"github.com/juju/worker/v3/workertest"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/agent"
	"github.com/juju/juju/api"
	apiprovisioner "github.com/juju/juju/api/agent/provisioner"
	apiserverprovisioner "github.com/juju/juju/apiserver/facades/agent/provisioner"
	"github.com/juju/juju/controller/authentication"
	"github.com/juju/juju/core/arch"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/instance"
	"github.com/juju/juju/core/life"
	"github.com/juju/juju/core/model"
	corenetwork "github.com/juju/juju/core/network"
	coreos "github.com/juju/juju/core/os"
	"github.com/juju/juju/core/os/ostype"
	"github.com/juju/juju/core/status"
	"github.com/juju/juju/environs"
	"github.com/juju/juju/environs/config"
	"github.com/juju/juju/environs/context"
	"github.com/juju/juju/environs/filestorage"
	"github.com/juju/juju/environs/imagemetadata"
	imagetesting "github.com/juju/juju/environs/imagemetadata/testing"
	"github.com/juju/juju/environs/instances"
	"github.com/juju/juju/environs/simplestreams"
	sstesting "github.com/juju/juju/environs/simplestreams/testing"
	envtesting "github.com/juju/juju/environs/testing"
	"github.com/juju/juju/environs/tools"
	"github.com/juju/juju/internal/worker/provisioner"
	"github.com/juju/juju/juju/testing"
	providercommon "github.com/juju/juju/provider/common"
	"github.com/juju/juju/provider/dummy"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/juju/state"
	"github.com/juju/juju/state/cloudimagemetadata"
	"github.com/juju/juju/storage"
	"github.com/juju/juju/storage/poolmanager"
	coretesting "github.com/juju/juju/testing"
	coretools "github.com/juju/juju/tools"
)

type CommonProvisionerSuite struct {
	testing.JujuConnSuite
	op  <-chan dummy.Operation
	cfg *config.Config
	// defaultConstraints are used when adding a machine and then later in test assertions.
	defaultConstraints constraints.Value

	st          api.Connection
	provisioner *apiprovisioner.State
	callCtx     context.ProviderCallContext
}

func (s *CommonProvisionerSuite) assertProvisionerObservesConfigChanges(c *gc.C, p provisioner.Provisioner) {
	// Inject our observer into the provisioner
	cfgObserver := make(chan *config.Config)
	provisioner.SetObserver(p, cfgObserver)

	// Switch to reaping on All machines.
	attrs := map[string]interface{}{
		config.ProvisionerHarvestModeKey: config.HarvestAll.String(),
	}
	err := s.Model.UpdateModelConfig(attrs, nil)
	c.Assert(err, jc.ErrorIsNil)

	// Wait for the PA to load the new configuration. We wait for the change we expect
	// like this because sometimes we pick up the initial harvest config (destroyed)
	// rather than the one we change to (all).
	var received []string
	timeout := time.After(coretesting.LongWait)
	for {
		select {
		case newCfg := <-cfgObserver:
			if newCfg.ProvisionerHarvestMode().String() == config.HarvestAll.String() {
				return
			}
			received = append(received, newCfg.ProvisionerHarvestMode().String())
		case <-time.After(coretesting.ShortWait):
		case <-timeout:
			if len(received) == 0 {
				c.Fatalf("PA did not action config change")
			} else {
				c.Fatalf("timed out waiting for config to change to '%s', received %+v",
					config.HarvestAll.String(), received)
			}
		}
	}
}

func (s *CommonProvisionerSuite) assertProvisionerObservesConfigChangesWorkerCount(c *gc.C, p provisioner.Provisioner, container bool) {
	// Inject our observer into the provisioner
	cfgObserver := make(chan *config.Config)
	provisioner.SetObserver(p, cfgObserver)

	// Switch to reaping on All machines.
	attrs := map[string]interface{}{}
	if container {
		attrs[config.NumContainerProvisionWorkersKey] = 10
	} else {
		attrs[config.NumProvisionWorkersKey] = 42
	}
	err := s.Model.UpdateModelConfig(attrs, nil)
	c.Assert(err, jc.ErrorIsNil)

	// Wait for the PA to load the new configuration. We wait for the change we expect
	// like this because sometimes we pick up the initial harvest config (destroyed)
	// rather than the one we change to (all).
	var received []int
	timeout := time.After(coretesting.LongWait)
	for {
		select {
		case newCfg := <-cfgObserver:
			if container {
				if newCfg.NumContainerProvisionWorkers() == 10 {
					return
				}
				received = append(received, newCfg.NumContainerProvisionWorkers())
			} else {
				if newCfg.NumProvisionWorkers() == 42 {
					return
				}
				received = append(received, newCfg.NumProvisionWorkers())
			}
		case <-timeout:
			if len(received) == 0 {
				c.Fatalf("PA did not action config change")
			} else {
				c.Fatalf("timed out waiting for config to change to '%s', received %+v",
					config.HarvestAll.String(), received)
			}
		}
	}
}

type ProvisionerSuite struct {
	CommonProvisionerSuite
}

var _ = gc.Suite(&ProvisionerSuite{})

func (s *CommonProvisionerSuite) SetUpSuite(c *gc.C) {
	s.JujuConnSuite.SetUpSuite(c)
	s.defaultConstraints = constraints.MustParse("arch=amd64 mem=4G cores=1 root-disk=8G")
}

func (s *CommonProvisionerSuite) SetUpTest(c *gc.C) {
	s.JujuConnSuite.SetUpTest(c)

	// We do not want to pull published image metadata for tests...
	imagetesting.PatchOfficialDataSources(&s.CleanupSuite, "")
	// We want an image to start test instances
	err := s.State.CloudImageMetadataStorage.SaveMetadata([]cloudimagemetadata.Metadata{{
		MetadataAttributes: cloudimagemetadata.MetadataAttributes{
			Region:          "region",
			Version:         "22.04",
			Arch:            "amd64",
			VirtType:        "",
			RootStorageType: "",
			Source:          "test",
			Stream:          "released",
		},
		Priority: 10,
		ImageId:  "-999",
	}})
	c.Assert(err, jc.ErrorIsNil)

	// Create the operations channel with more than enough space
	// for those tests that don't listen on it.
	op := make(chan dummy.Operation, 500)
	dummy.Listen(op)
	s.op = op

	cfg, err := s.Model.ModelConfig()
	c.Assert(err, jc.ErrorIsNil)
	s.cfg = cfg

	s.callCtx = context.NewEmptyCloudCallContext()

	// Create a machine for the dummy bootstrap instance,
	// so the provisioner doesn't destroy it.
	insts, err := s.Environ.Instances(s.callCtx, []instance.Id{dummy.BootstrapInstanceId})
	c.Assert(err, jc.ErrorIsNil)
	addrs, err := insts[0].Addresses(s.callCtx)
	c.Assert(err, jc.ErrorIsNil)

	pAddrs := make(corenetwork.SpaceAddresses, len(addrs))
	for i, addr := range addrs {
		pAddrs[i] = corenetwork.SpaceAddress{MachineAddress: addr.MachineAddress}
	}

	machine, err := s.State.AddOneMachine(state.MachineTemplate{
		Addresses:  pAddrs,
		Base:       state.UbuntuBase("12.10"),
		Nonce:      agent.BootstrapNonce,
		InstanceId: dummy.BootstrapInstanceId,
		Jobs:       []state.MachineJob{state.JobManageModel},
	})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(machine.Id(), gc.Equals, "0")

	current := coretesting.CurrentVersion()
	err = machine.SetAgentVersion(current)
	c.Assert(err, jc.ErrorIsNil)

	password, err := utils.RandomPassword()
	c.Assert(err, jc.ErrorIsNil)
	err = machine.SetPassword(password)
	c.Assert(err, jc.ErrorIsNil)

	s.st = s.OpenAPIAsMachine(c, machine.Tag(), password, agent.BootstrapNonce)
	c.Assert(s.st, gc.NotNil)
	c.Logf("API: login as %q successful", machine.Tag())
	s.provisioner = apiprovisioner.NewState(s.st)
	c.Assert(s.provisioner, gc.NotNil)

}

func (s *CommonProvisionerSuite) startUnknownInstance(c *gc.C, id string) instances.Instance {
	instance, _ := testing.AssertStartInstance(c, s.Environ, s.callCtx, s.ControllerConfig.ControllerUUID(), id)
	select {
	case o := <-s.op:
		switch o := o.(type) {
		case dummy.OpStartInstance:
		default:
			c.Fatalf("unexpected operation %#v", o)
		}
	case <-time.After(coretesting.LongWait):
		c.Fatalf("timed out waiting for startinstance operation")
	}
	return instance
}

func (s *CommonProvisionerSuite) checkStartInstance(c *gc.C, m *state.Machine) instances.Instance {
	retVal := s.checkStartInstancesCustom(c, []*state.Machine{m}, "pork", s.defaultConstraints,
		nil, nil, nil, nil, nil, nil, true)
	return retVal[m.Id()]
}

func (s *CommonProvisionerSuite) checkStartInstanceCustom(
	c *gc.C, m *state.Machine,
	secret string, cons constraints.Value,
	networkInfo corenetwork.InterfaceInfos,
	subnetsToZones map[corenetwork.Id][]string,
	rootDisk *storage.VolumeParams,
	volumes []storage.Volume,
	volumeAttachments []storage.VolumeAttachment,
	checkPossibleTools coretools.List,
	waitInstanceId bool,
) instances.Instance {
	retVal := s.checkStartInstancesCustom(c, []*state.Machine{m},
		secret, cons, networkInfo, subnetsToZones, rootDisk, volumes,
		volumeAttachments, checkPossibleTools, waitInstanceId)
	return retVal[m.Id()]
}

func (s *CommonProvisionerSuite) checkStartInstances(c *gc.C, machines []*state.Machine) map[string]instances.Instance {
	return s.checkStartInstancesCustom(c, machines, "pork", s.defaultConstraints, nil, nil,
		nil, nil, nil, nil, true)
}

// checkStartInstanceCustom takes a slice of Machines.  A
// map of machine Ids to instances is returned
func (s *CommonProvisionerSuite) checkStartInstancesCustom(
	c *gc.C, machines []*state.Machine,
	secret string, cons constraints.Value,
	networkInfo corenetwork.InterfaceInfos,
	subnetsToZones map[corenetwork.Id][]string,
	rootDisk *storage.VolumeParams,
	volumes []storage.Volume,
	volumeAttachments []storage.VolumeAttachment,
	checkPossibleTools coretools.List,
	waitInstanceId bool,
) (
	returnInstances map[string]instances.Instance,
) {
	returnInstances = make(map[string]instances.Instance, len(machines))
	found := 0
	for {
		select {
		case o := <-s.op:
			switch o := o.(type) {
			case dummy.OpStartInstance:
				inst := o.Instance

				var m *state.Machine
				for _, machine := range machines {
					if machine.Id() == o.MachineId {
						m = machine
						found += 1
						break
					}
				}
				c.Assert(m, gc.NotNil)
				if waitInstanceId {
					s.waitInstanceId(c, m, inst.Id())
				}

				// Check the instance was started with the expected params.
				c.Assert(o.MachineId, gc.Equals, m.Id())
				nonceParts := strings.SplitN(o.MachineNonce, ":", 2)
				c.Assert(nonceParts, gc.HasLen, 2)
				c.Assert(nonceParts[0], gc.Equals, names.NewMachineTag("0").String())
				c.Assert(nonceParts[1], jc.Satisfies, utils.IsValidUUIDString)
				c.Assert(o.Secret, gc.Equals, secret)
				c.Assert(o.SubnetsToZones, jc.DeepEquals, subnetsToZones)
				c.Assert(o.NetworkInfo, jc.DeepEquals, networkInfo)
				c.Assert(o.RootDisk, jc.DeepEquals, rootDisk)
				c.Assert(o.Volumes, jc.DeepEquals, volumes)
				c.Assert(o.VolumeAttachments, jc.DeepEquals, volumeAttachments)

				var jobs []model.MachineJob
				for _, job := range m.Jobs() {
					jobs = append(jobs, job.ToParams())
				}
				c.Assert(o.Jobs, jc.SameContents, jobs)

				if checkPossibleTools != nil {
					for _, t := range o.PossibleTools {
						url := s.st.Addr()
						url.Scheme = "https"
						url.Path = path.Join(url.Path, "model", coretesting.ModelTag.Id(), "tools", t.Version.String())
						c.Check(t.URL, gc.Equals, url.String())
						t.URL = ""
					}
					for _, t := range checkPossibleTools {
						t.URL = ""
					}
					c.Assert(o.PossibleTools, gc.DeepEquals, checkPossibleTools)
				}

				// All provisioned machines in this test suite have
				// their hardware characteristics attributes set to
				// the same values as the constraints due to the dummy
				// environment being used.
				if !constraints.IsEmpty(&cons) {
					c.Assert(o.Constraints, gc.DeepEquals, cons)
					hc, err := m.HardwareCharacteristics()
					c.Assert(err, jc.ErrorIsNil)
					// At this point we don't care what the AvailabilityZone is,
					// it can be a few different valid things.
					zone := hc.AvailabilityZone
					hc.AvailabilityZone = nil
					c.Assert(*hc, gc.DeepEquals, instance.HardwareCharacteristics{
						Arch:     cons.Arch,
						Mem:      cons.Mem,
						RootDisk: cons.RootDisk,
						CpuCores: cons.CpuCores,
						CpuPower: cons.CpuPower,
						Tags:     cons.Tags,
					})
					hc.AvailabilityZone = zone
				}
				returnInstances[m.Id()] = inst
				if found == len(machines) {
					return
				}
				break
			default:
				c.Logf("ignoring unexpected operation %#v", o)
			}
		case <-time.After(2 * time.Second):
			c.Fatalf("provisioner did not start an instance")
			return
		}
	}
}

// checkNoOperations checks that the environ was not operated upon.
func (s *CommonProvisionerSuite) checkNoOperations(c *gc.C) {
	select {
	case o := <-s.op:
		c.Fatalf("unexpected operation %+v", o)
	case <-time.After(coretesting.ShortWait):
		return
	}
}

// checkStopInstances checks that an instance has been stopped.
func (s *CommonProvisionerSuite) checkStopInstances(c *gc.C, instances ...instances.Instance) {
	s.checkStopSomeInstances(c, instances, nil)
}

// checkStopSomeInstances checks that instancesToStop are stopped while instancesToKeep are not.
func (s *CommonProvisionerSuite) checkStopSomeInstances(c *gc.C,
	instancesToStop []instances.Instance, instancesToKeep []instances.Instance) {

	instanceIdsToStop := set.NewStrings()
	for _, instance := range instancesToStop {
		instanceIdsToStop.Add(string(instance.Id()))
	}
	instanceIdsToKeep := set.NewStrings()
	for _, instance := range instancesToKeep {
		instanceIdsToKeep.Add(string(instance.Id()))
	}
	// Continue checking for stop instance calls until all the instances we
	// are waiting on to finish, actually finish, or we time out.
	for !instanceIdsToStop.IsEmpty() {
		select {
		case o := <-s.op:
			switch o := o.(type) {
			case dummy.OpStopInstances:
				for _, id := range o.Ids {
					instId := string(id)
					instanceIdsToStop.Remove(instId)
					if instanceIdsToKeep.Contains(instId) {
						c.Errorf("provisioner unexpectedly stopped instance %s", instId)
					}
				}
			default:
				c.Fatalf("unexpected operation %#v", o)
				return
			}
		case <-time.After(2 * time.Second):
			c.Fatalf("provisioner did not stop an instance")
			return
		}
	}
}

func (s *CommonProvisionerSuite) waitForWatcher(c *gc.C, w state.NotifyWatcher, name string, check func() bool) {
	// TODO(jam): We need to grow a new method on NotifyWatcherC
	// that calls StartSync while waiting for changes, then
	// waitMachine and waitHardwareCharacteristics can use that
	// instead
	defer workertest.CleanKill(c, w)
	timeout := time.After(coretesting.LongWait)
	resync := time.After(0)
	for {
		select {
		case <-w.Changes():
			if check() {
				return
			}
		case <-resync:
			resync = time.After(coretesting.ShortWait)

		case <-timeout:
			c.Fatalf("%v wait timed out", name)
		}
	}
}

func (s *CommonProvisionerSuite) waitHardwareCharacteristics(c *gc.C, m *state.Machine, check func() bool) {
	w := m.WatchInstanceData()
	name := fmt.Sprintf("hardware characteristics for machine %v", m)
	s.waitForWatcher(c, w, name, check)
}

// waitForRemovalMark waits for the supplied machine to be marked for removal.
func (s *CommonProvisionerSuite) waitForRemovalMark(c *gc.C, m *state.Machine) {
	w := s.BackingState.WatchMachineRemovals()
	name := fmt.Sprintf("machine %v marked for removal", m)
	s.waitForWatcher(c, w, name, func() bool {
		removals, err := s.BackingState.AllMachineRemovals()
		c.Assert(err, jc.ErrorIsNil)
		for _, removal := range removals {
			if removal == m.Id() {
				return true
			}
		}
		return false
	})
}

// waitInstanceId waits until the supplied machine has an instance id, then
// asserts it is as expected.
func (s *CommonProvisionerSuite) waitInstanceId(c *gc.C, m *state.Machine, expect instance.Id) {
	s.waitHardwareCharacteristics(c, m, func() bool {
		if actual, err := m.InstanceId(); err == nil {
			c.Assert(actual, gc.Equals, expect)
			return true
		} else if !errors.IsNotProvisioned(err) {
			// We don't expect any errors.
			panic(err)
		} else {
			c.Logf("got not provisioned error while waiting: %v", err)
		}
		return false
	})
}

func (s *CommonProvisionerSuite) newEnvironProvisioner(c *gc.C) provisioner.Provisioner {
	machineTag := names.NewMachineTag("0")
	agentConfig := s.AgentConfigForTag(c, machineTag)
	apiState := apiprovisioner.NewState(s.st)
	w, err := provisioner.NewEnvironProvisioner(apiState, agentConfig, loggo.GetLogger("test"), s.Environ, &credentialAPIForTest{})
	c.Assert(err, jc.ErrorIsNil)
	return w
}

func (s *CommonProvisionerSuite) addMachine() (*state.Machine, error) {
	return s.addMachineWithConstraints(s.defaultConstraints)
}

func (s *CommonProvisionerSuite) addMachineWithConstraints(cons constraints.Value) (*state.Machine, error) {
	return s.BackingState.AddOneMachine(state.MachineTemplate{
		Base:        state.DefaultLTSBase(),
		Jobs:        []state.MachineJob{state.JobHostUnits},
		Constraints: cons,
	})
}

func (s *CommonProvisionerSuite) addMachines(number int) ([]*state.Machine, error) {
	templates := make([]state.MachineTemplate, number)
	for i := range templates {
		templates[i] = state.MachineTemplate{
			Base:        state.DefaultLTSBase(),
			Jobs:        []state.MachineJob{state.JobHostUnits},
			Constraints: s.defaultConstraints,
		}
	}
	return s.BackingState.AddMachines(templates...)
}

func (s *CommonProvisionerSuite) enableHA(c *gc.C, n int) []*state.Machine {
	changes, err := s.BackingState.EnableHA(n, s.defaultConstraints, state.DefaultLTSBase(), nil)
	c.Assert(err, jc.ErrorIsNil)
	added := make([]*state.Machine, len(changes.Added))
	for i, mid := range changes.Added {
		m, err := s.BackingState.Machine(mid)
		c.Assert(err, jc.ErrorIsNil)
		added[i] = m
	}
	return added
}

func (s *ProvisionerSuite) TestProvisionerStartStop(c *gc.C) {
	p := s.newEnvironProvisioner(c)
	workertest.CleanKill(c, p)
}

func (s *ProvisionerSuite) TestSimple(c *gc.C) {
	p := s.newEnvironProvisioner(c)
	defer workertest.CleanKill(c, p)

	// Check that an instance is provisioned when the machine is created...
	m, err := s.addMachine()
	c.Assert(err, jc.ErrorIsNil)
	instance := s.checkStartInstance(c, m)

	// ...and removed, along with the machine, when the machine is Dead.
	c.Assert(m.EnsureDead(), gc.IsNil)
	s.checkStopInstances(c, instance)
	s.waitForRemovalMark(c, m)
}

func (s *ProvisionerSuite) TestConstraints(c *gc.C) {
	// Create a machine with non-standard constraints.
	m, err := s.addMachine()
	c.Assert(err, jc.ErrorIsNil)
	cons := constraints.MustParse("mem=8G arch=amd64 cores=2 root-disk=10G")
	err = m.SetConstraints(cons)
	c.Assert(err, jc.ErrorIsNil)

	// Start a provisioner and check those constraints are used.
	p := s.newEnvironProvisioner(c)
	defer workertest.CleanKill(c, p)

	s.checkStartInstanceCustom(c, m, "pork", cons, nil, nil, nil, nil, nil, nil, true)
}

func (s *ProvisionerSuite) TestPossibleTools(c *gc.C) {

	storageDir := c.MkDir()
	s.PatchValue(&tools.DefaultBaseURL, storageDir)
	stor, err := filestorage.NewFileStorageWriter(storageDir)
	c.Assert(err, jc.ErrorIsNil)
	currentVersion := version.MustParseBinary("1.2.3-ubuntu-amd64")

	// The current version is determined by the current model's agent
	// version when locating tools to provision an added unit
	attrs := map[string]interface{}{
		config.AgentVersionKey: currentVersion.Number.String(),
	}
	err = s.Model.UpdateModelConfig(attrs, nil)
	c.Assert(err, jc.ErrorIsNil)

	s.PatchValue(&arch.HostArch, func() string { return currentVersion.Arch })
	s.PatchValue(&coreos.HostOS, func() ostype.OSType { return ostype.Ubuntu })

	// Upload some plausible matches, and some that should be filtered out.
	compatibleVersion := version.MustParseBinary("1.2.3-quantal-arm64")
	ignoreVersion1 := version.MustParseBinary("1.2.4-ubuntu-arm64")
	ignoreVersion2 := version.MustParseBinary("1.2.3-windows-arm64")
	availableVersions := []version.Binary{
		currentVersion, compatibleVersion, ignoreVersion1, ignoreVersion2,
	}
	envtesting.AssertUploadFakeToolsVersions(c, stor, s.cfg.AgentStream(), s.cfg.AgentStream(), availableVersions...)

	// Extract the tools that we expect to actually match.
	ss := simplestreams.NewSimpleStreams(sstesting.TestDataSourceFactory())
	expectedList, err := tools.FindTools(ss, s.Environ, -1, -1, []string{s.cfg.AgentStream()}, coretools.Filter{
		Number: currentVersion.Number,
		OSType: "ubuntu",
	})
	c.Assert(err, jc.ErrorIsNil)

	// Create the machine and check the tools that get passed into StartInstance.
	machine, err := s.BackingState.AddOneMachine(state.MachineTemplate{
		Base: state.UbuntuBase("12.10"),
		Jobs: []state.MachineJob{state.JobHostUnits},
	})
	c.Assert(err, jc.ErrorIsNil)

	provisioner := s.newEnvironProvisioner(c)
	defer workertest.CleanKill(c, provisioner)
	s.checkStartInstanceCustom(
		c, machine, "pork", constraints.Value{},
		nil, nil, nil, nil, nil, expectedList, true,
	)
}

var validCloudInitUserData = `
packages:
  - 'python-keystoneclient'
  - 'python-glanceclient'
preruncmd:
  - mkdir /tmp/preruncmd
  - mkdir /tmp/preruncmd2
postruncmd:
  - mkdir /tmp/postruncmd
  - mkdir /tmp/postruncmd2
package_upgrade: false
`[1:]

func (s *ProvisionerSuite) TestSetUpToStartMachine(c *gc.C) {
	attrs := map[string]interface{}{
		config.CloudInitUserDataKey: validCloudInitUserData,
	}

	err := s.Model.UpdateModelConfig(attrs, nil)
	c.Assert(err, jc.ErrorIsNil)

	task := s.newProvisionerTask(
		c,
		config.HarvestAll,
		s.Environ,
		s.provisioner,
		&mockDistributionGroupFinder{},
		mockToolsFinder{},
	)
	defer workertest.CleanKill(c, task)

	machine, err := s.addMachine()
	c.Assert(err, jc.ErrorIsNil)

	mRes, err := s.provisioner.Machines(machine.MachineTag())
	c.Assert(err, gc.IsNil)
	c.Assert(mRes, gc.HasLen, 1)
	c.Assert(mRes[0].Err, gc.IsNil)
	apiMachine := mRes[0].Machine

	pRes, err := s.provisioner.ProvisioningInfo([]names.MachineTag{machine.MachineTag()})
	c.Assert(err, gc.IsNil)
	c.Assert(pRes.Results, gc.HasLen, 1)

	v, err := apiMachine.ModelAgentVersion()
	c.Assert(err, jc.ErrorIsNil)

	startInstanceParams, err := provisioner.SetupToStartMachine(task, apiMachine, v, pRes.Results[0])
	c.Assert(err, jc.ErrorIsNil)
	cloudInitUserData := startInstanceParams.InstanceConfig.CloudInitUserData
	c.Assert(cloudInitUserData, gc.DeepEquals, map[string]interface{}{
		"packages":        []interface{}{"python-keystoneclient", "python-glanceclient"},
		"preruncmd":       []interface{}{"mkdir /tmp/preruncmd", "mkdir /tmp/preruncmd2"},
		"postruncmd":      []interface{}{"mkdir /tmp/postruncmd", "mkdir /tmp/postruncmd2"},
		"package_upgrade": false},
	)
}

func (s *ProvisionerSuite) TestProvisionerSetsErrorStatusWhenNoToolsAreAvailable(c *gc.C) {
	p := s.newEnvironProvisioner(c)
	defer workertest.CleanKill(c, p)

	// Check that an instance is not provisioned when the machine is created...
	m, err := s.BackingState.AddOneMachine(state.MachineTemplate{
		// We need a valid series that has no tools uploaded
		Base:        state.Base{OS: "centos", Channel: "7"},
		Jobs:        []state.MachineJob{state.JobHostUnits},
		Constraints: s.defaultConstraints,
	})
	c.Assert(err, jc.ErrorIsNil)
	s.checkNoOperations(c)

	// Ensure machine error status was set, and the error matches
	agentStatus, instanceStatus := s.waitUntilMachineNotPending(c, m)
	c.Check(agentStatus.Status, gc.Equals, status.Error)
	c.Check(agentStatus.Message, gc.Equals, "no matching agent binaries available")
	c.Check(instanceStatus.Status, gc.Equals, status.ProvisioningError)
	c.Check(instanceStatus.Message, gc.Equals, "no matching agent binaries available")

	// Restart the PA to make sure the machine is skipped again.
	workertest.CleanKill(c, p)
	p = s.newEnvironProvisioner(c)
	defer workertest.CleanKill(c, p)
	s.checkNoOperations(c)
}

func (s *ProvisionerSuite) waitUntilMachineNotPending(c *gc.C, m *state.Machine) (status.StatusInfo, status.StatusInfo) {
	t0 := time.Now()
	for time.Since(t0) < 10*coretesting.LongWait {
		agentStatusInfo, err := m.Status()
		c.Assert(err, jc.ErrorIsNil)
		if agentStatusInfo.Status == status.Pending {
			time.Sleep(coretesting.ShortWait)
			continue
		}
		instanceStatusInfo, err := m.InstanceStatus()
		c.Assert(err, jc.ErrorIsNil)
		// officially InstanceStatus is only supposed to be Provisioning, but
		// all current Providers have their unknown state as Pending.
		if instanceStatusInfo.Status == status.Provisioning ||
			instanceStatusInfo.Status == status.Pending {
			time.Sleep(coretesting.ShortWait)
			continue
		}
		return agentStatusInfo, instanceStatusInfo
	}
	c.Fatalf("machine %q stayed in pending", m.Id())
	// Satisfy Go, Fatal should be a panic anyway
	return status.StatusInfo{}, status.StatusInfo{}
}

func (s *ProvisionerSuite) TestProvisionerFailedStartInstanceWithInjectedCreationError(c *gc.C) {
	// Set the retry delay to 0, and retry count to 2 to keep tests short
	s.PatchValue(provisioner.RetryStrategyDelay, 0*time.Second)
	s.PatchValue(provisioner.RetryStrategyCount, 2)

	// create the error injection channel
	errorInjectionChannel := make(chan error, 3)

	p := s.newEnvironProvisioner(c)
	defer workertest.CleanKill(c, p)

	// patch the dummy provider error injection channel
	cleanup := dummy.PatchTransientErrorInjectionChannel(errorInjectionChannel)
	defer cleanup()

	retryableError := environs.ZoneIndependentError(
		errors.New("container failed to start and was destroyed"),
	)
	destroyError := environs.ZoneIndependentError(
		errors.New("container failed to start and failed to destroy: manual cleanup of containers needed"),
	)
	// send the error message three times, because the provisioner will retry twice as patched above.
	errorInjectionChannel <- retryableError
	errorInjectionChannel <- retryableError
	errorInjectionChannel <- destroyError

	m, err := s.addMachine()
	c.Assert(err, jc.ErrorIsNil)
	s.checkNoOperations(c)

	agentStatus, instanceStatus := s.waitUntilMachineNotPending(c, m)
	// check that the status matches the error message
	c.Check(agentStatus.Status, gc.Equals, status.Error)
	c.Check(agentStatus.Message, gc.Equals, destroyError.Error())
	c.Check(instanceStatus.Status, gc.Equals, status.ProvisioningError)
	c.Check(instanceStatus.Message, gc.Equals, destroyError.Error())
}

func (s *ProvisionerSuite) TestProvisionerSucceedStartInstanceWithInjectedRetryableCreationError(c *gc.C) {
	// Set the retry delay to 0, and retry count to 2 to keep tests short
	s.PatchValue(provisioner.RetryStrategyDelay, 0*time.Second)
	s.PatchValue(provisioner.RetryStrategyCount, 2)

	// create the error injection channel
	errorInjectionChannel := make(chan error, 1)
	c.Assert(errorInjectionChannel, gc.NotNil)

	p := s.newEnvironProvisioner(c)
	defer workertest.CleanKill(c, p)

	// patch the dummy provider error injection channel
	cleanup := dummy.PatchTransientErrorInjectionChannel(errorInjectionChannel)
	defer cleanup()

	// send the error message once
	// - instance creation should succeed
	retryableError := errors.New("container failed to start and was destroyed")
	errorInjectionChannel <- retryableError

	m, err := s.addMachine()
	c.Assert(err, jc.ErrorIsNil)
	s.checkStartInstance(c, m)
}

func (s *ProvisionerSuite) TestProvisionerStopRetryingIfDying(c *gc.C) {
	// Create the error injection channel and inject
	// a retryable error
	errorInjectionChannel := make(chan error, 1)

	p := s.newEnvironProvisioner(c)
	// Don't refer the stop.  We will manually stop and verify the result.

	// patch the dummy provider error injection channel
	cleanup := dummy.PatchTransientErrorInjectionChannel(errorInjectionChannel)
	defer cleanup()

	retryableError := errors.New("container failed to start and was destroyed")
	errorInjectionChannel <- retryableError

	m, err := s.addMachine()
	c.Assert(err, jc.ErrorIsNil)

	time.Sleep(coretesting.ShortWait)

	workertest.CleanKill(c, p)
	statusInfo, err := m.Status()
	c.Assert(err, jc.ErrorIsNil)
	c.Check(statusInfo.Status, gc.Equals, status.Pending)
	statusInfo, err = m.InstanceStatus()
	c.Assert(err, jc.ErrorIsNil)
	if statusInfo.Status != status.Pending && statusInfo.Status != status.Provisioning {
		c.Errorf("statusInfo.Status was %q not one of %q or %q",
			statusInfo.Status, status.Pending, status.Provisioning)
	}
	s.checkNoOperations(c)
}

func (s *ProvisionerSuite) TestProvisioningDoesNotOccurForLXD(c *gc.C) {
	p := s.newEnvironProvisioner(c)
	defer workertest.CleanKill(c, p)

	// create a machine to host the container.
	m, err := s.addMachine()
	c.Assert(err, jc.ErrorIsNil)
	inst := s.checkStartInstance(c, m)

	// make a container on the machine we just created
	template := state.MachineTemplate{
		Base: state.DefaultLTSBase(),
		Jobs: []state.MachineJob{state.JobHostUnits},
	}
	container, err := s.State.AddMachineInsideMachine(template, m.Id(), instance.LXD)
	c.Assert(err, jc.ErrorIsNil)

	// the PA should not attempt to create it
	s.checkNoOperations(c)

	// cleanup
	c.Assert(container.EnsureDead(), gc.IsNil)
	c.Assert(container.Remove(), gc.IsNil)
	c.Assert(m.EnsureDead(), gc.IsNil)
	s.checkStopInstances(c, inst)
	s.waitForRemovalMark(c, m)
}

func (s *ProvisionerSuite) TestProvisioningDoesNotOccurForKVM(c *gc.C) {
	p := s.newEnvironProvisioner(c)
	defer workertest.CleanKill(c, p)

	// create a machine to host the container.
	m, err := s.addMachine()
	c.Assert(err, jc.ErrorIsNil)
	inst := s.checkStartInstance(c, m)

	// make a container on the machine we just created
	template := state.MachineTemplate{
		Base: state.DefaultLTSBase(),
		Jobs: []state.MachineJob{state.JobHostUnits},
	}
	container, err := s.State.AddMachineInsideMachine(template, m.Id(), instance.KVM)
	c.Assert(err, jc.ErrorIsNil)

	// the PA should not attempt to create it
	s.checkNoOperations(c)

	// cleanup
	c.Assert(container.EnsureDead(), gc.IsNil)
	c.Assert(container.Remove(), gc.IsNil)
	c.Assert(m.EnsureDead(), gc.IsNil)
	s.checkStopInstances(c, inst)
	s.waitForRemovalMark(c, m)
}

type MachineClassifySuite struct {
}

var _ = gc.Suite(&MachineClassifySuite{})

type MockMachine struct {
	life          life.Value
	status        status.Status
	id            string
	idErr         error
	ensureDeadErr error
	statusErr     error
}

func (m *MockMachine) Life() life.Value {
	return m.life
}

func (m *MockMachine) InstanceId() (instance.Id, error) {
	return instance.Id(m.id), m.idErr
}

func (m *MockMachine) InstanceNames() (instance.Id, string, error) {
	instId, err := m.InstanceId()
	return instId, "", err
}

func (m *MockMachine) EnsureDead() error {
	return m.ensureDeadErr
}

func (m *MockMachine) Status() (status.Status, string, error) {
	return m.status, "", m.statusErr
}

func (m *MockMachine) InstanceStatus() (status.Status, string, error) {
	return m.status, "", m.statusErr
}

func (m *MockMachine) Id() string {
	return m.id
}

type machineClassificationTest struct {
	description    string
	life           life.Value
	status         status.Status
	idErr          string
	ensureDeadErr  string
	expectErrCode  string
	expectErrFmt   string
	statusErr      string
	classification provisioner.MachineClassification
}

var machineClassificationTestsNoMaintenance = machineClassificationTest{
	description:    "Machine doesn't need maintaining",
	life:           life.Alive,
	status:         status.Started,
	classification: provisioner.None,
}

func (s *MachineClassifySuite) TestMachineClassification(c *gc.C) {
	test := func(t machineClassificationTest, id string) {
		// Run a sub-test from the test table
		s2e := func(s string) error {
			// Little helper to turn a non-empty string into a useful error for "ErrorMatches"
			if s != "" {
				return &params.Error{Code: s}
			}
			return nil
		}

		c.Logf("%s: %s", id, t.description)
		machine := MockMachine{t.life, t.status, id, s2e(t.idErr), s2e(t.ensureDeadErr), s2e(t.statusErr)}
		classification, err := provisioner.ClassifyMachine(loggo.GetLogger("test"), &machine)
		if err != nil {
			c.Assert(err, gc.ErrorMatches, fmt.Sprintf(t.expectErrFmt, machine.Id()))
		} else {
			c.Assert(err, gc.Equals, s2e(t.expectErrCode))
		}
		c.Assert(classification, gc.Equals, t.classification)
	}

	test(machineClassificationTestsNoMaintenance, "0")
}

func (s *ProvisionerSuite) TestProvisioningMachinesWithSpacesSuccess(c *gc.C) {
	p := s.newEnvironProvisioner(c)
	defer workertest.CleanKill(c, p)

	// Add the spaces used in constraints.
	space1, err := s.State.AddSpace("space1", "", nil, false)
	c.Assert(err, jc.ErrorIsNil)
	space2, err := s.State.AddSpace("space2", "", nil, false)
	c.Assert(err, jc.ErrorIsNil)

	// Add 1 subnet into space1, and 2 into space2.
	// Each subnet is in a matching zone (e.g "subnet-#" in "zone#").
	testing.AddSubnetsWithTemplate(c, s.State, 3, corenetwork.SubnetInfo{
		CIDR:              "10.10.{{.}}.0/24",
		ProviderId:        "subnet-{{.}}",
		AvailabilityZones: []string{"zone{{.}}"},
		SpaceID:           fmt.Sprintf("{{if (lt . 2)}}%s{{else}}%s{{end}}", space1.Id(), space2.Id()),
		VLANTag:           42,
	})

	// Add and provision a machine with spaces specified.
	cons := constraints.MustParse(
		s.defaultConstraints.String(), "spaces=space2,^space1",
	)
	// The dummy provider simulates 2 subnets per included space.
	expectedSubnetsToZones := map[corenetwork.Id][]string{
		"subnet-0": {"zone0"},
		"subnet-1": {"zone1"},
	}
	m, err := s.addMachineWithConstraints(cons)
	c.Assert(err, jc.ErrorIsNil)
	inst := s.checkStartInstanceCustom(
		c, m, "pork", cons,
		nil,
		expectedSubnetsToZones,
		nil, nil, nil, nil, true,
	)

	// Cleanup.
	c.Assert(m.EnsureDead(), gc.IsNil)
	s.checkStopInstances(c, inst)
	s.waitForRemovalMark(c, m)
}

func (s *ProvisionerSuite) testProvisioningFailsAndSetsErrorStatusForConstraints(
	c *gc.C,
	cons constraints.Value,
	expectedErrorStatus string,
) {
	machine, err := s.addMachineWithConstraints(cons)
	c.Assert(err, jc.ErrorIsNil)

	// Start the PA.
	p := s.newEnvironProvisioner(c)
	defer workertest.CleanKill(c, p)

	// Expect StartInstance to fail.
	s.checkNoOperations(c)

	// Ensure machine error status was set, and the error matches
	agentStatus, instanceStatus := s.waitUntilMachineNotPending(c, machine)
	c.Check(agentStatus.Status, gc.Equals, status.Error)
	c.Check(agentStatus.Message, gc.Equals, expectedErrorStatus)
	c.Check(instanceStatus.Status, gc.Equals, status.ProvisioningError)
	c.Check(instanceStatus.Message, gc.Equals, expectedErrorStatus)

	// Make sure the task didn't stop with an error
	died := make(chan error)
	go func() {
		died <- p.Wait()
	}()
	select {
	case <-time.After(coretesting.ShortWait):
	case err := <-died:
		c.Fatalf("provisioner task died unexpectedly with err: %v", err)
	}

	// Restart the PA to make sure the machine is not retried.
	workertest.CleanKill(c, p)
	p = s.newEnvironProvisioner(c)
	defer workertest.CleanKill(c, p)

	s.checkNoOperations(c)
}

func (s *ProvisionerSuite) TestProvisioningMachinesFailsWithUnknownSpaces(c *gc.C) {
	cons := constraints.MustParse(
		s.defaultConstraints.String(), "spaces=missing,missing-too,^ignored-too",
	)
	expectedErrorStatus := `matching subnets to zones: space "missing" not found`
	s.testProvisioningFailsAndSetsErrorStatusForConstraints(c, cons, expectedErrorStatus)
}

func (s *ProvisionerSuite) TestProvisioningMachinesFailsWithEmptySpaces(c *gc.C) {
	_, err := s.State.AddSpace("empty", "", nil, false)
	c.Assert(err, jc.ErrorIsNil)
	cons := constraints.MustParse(
		s.defaultConstraints.String(), "spaces=empty",
	)
	expectedErrorStatus := `matching subnets to zones: ` +
		`cannot use space "empty" as deployment target: no subnets`
	s.testProvisioningFailsAndSetsErrorStatusForConstraints(c, cons, expectedErrorStatus)
}

func (s *CommonProvisionerSuite) addMachineWithRequestedVolumes(volumes []state.HostVolumeParams, cons constraints.Value) (*state.Machine, error) {
	return s.BackingState.AddOneMachine(state.MachineTemplate{
		Base:        state.DefaultLTSBase(),
		Jobs:        []state.MachineJob{state.JobHostUnits},
		Constraints: cons,
		Volumes:     volumes,
	})
}

func (s *ProvisionerSuite) TestProvisioningMachinesWithRequestedRootDisk(c *gc.C) {
	// Set up a persistent pool.
	poolManager := poolmanager.New(state.NewStateSettings(s.State), s.Environ)
	_, err := poolManager.Create("persistent-pool", "static", map[string]interface{}{"persistent": true})
	c.Assert(err, jc.ErrorIsNil)

	p := s.newEnvironProvisioner(c)
	defer workertest.CleanKill(c, p)

	cons := constraints.MustParse("root-disk-source=persistent-pool " + s.defaultConstraints.String())
	m, err := s.BackingState.AddOneMachine(state.MachineTemplate{
		Base:        state.DefaultLTSBase(),
		Jobs:        []state.MachineJob{state.JobHostUnits},
		Constraints: cons,
	})
	c.Assert(err, jc.ErrorIsNil)

	inst := s.checkStartInstanceCustom(
		c, m, "pork", cons,
		nil, nil,
		&storage.VolumeParams{
			Provider:   "static",
			Attributes: map[string]interface{}{"persistent": true},
		},
		nil,
		nil,
		nil, true,
	)

	// Cleanup.
	c.Assert(m.EnsureDead(), gc.IsNil)
	s.checkStopInstances(c, inst)
	s.waitForRemovalMark(c, m)
}

func (s *ProvisionerSuite) TestProvisioningMachinesWithRequestedVolumes(c *gc.C) {
	// Set up a persistent pool.
	poolManager := poolmanager.New(state.NewStateSettings(s.State), s.Environ)
	_, err := poolManager.Create("persistent-pool", "static", map[string]interface{}{"persistent": true})
	c.Assert(err, jc.ErrorIsNil)

	p := s.newEnvironProvisioner(c)
	defer workertest.CleanKill(c, p)

	// Add a machine with volumes to state.
	requestedVolumes := []state.HostVolumeParams{{
		Volume:     state.VolumeParams{Pool: "static", Size: 1024},
		Attachment: state.VolumeAttachmentParams{},
	}, {
		Volume:     state.VolumeParams{Pool: "persistent-pool", Size: 2048},
		Attachment: state.VolumeAttachmentParams{},
	}, {
		Volume:     state.VolumeParams{Pool: "persistent-pool", Size: 4096},
		Attachment: state.VolumeAttachmentParams{},
	}}
	m, err := s.addMachineWithRequestedVolumes(requestedVolumes, s.defaultConstraints)
	c.Assert(err, jc.ErrorIsNil)

	// Provision volume-2, so that it is attached rather than created.
	sb, err := state.NewStorageBackend(s.State)
	c.Assert(err, jc.ErrorIsNil)
	err = sb.SetVolumeInfo(names.NewVolumeTag("2"), state.VolumeInfo{
		Pool:     "persistent-pool",
		VolumeId: "vol-ume",
		Size:     4096,
	})
	c.Assert(err, jc.ErrorIsNil)

	// Provision the machine, checking the volume and volume attachment arguments.
	expectedVolumes := []storage.Volume{{
		names.NewVolumeTag("0"),
		storage.VolumeInfo{
			Size: 1024,
		},
	}, {
		names.NewVolumeTag("1"),
		storage.VolumeInfo{
			Size:       2048,
			Persistent: true,
		},
	}}
	expectedVolumeAttachments := []storage.VolumeAttachment{{
		Volume:  names.NewVolumeTag("2"),
		Machine: m.MachineTag(),
		VolumeAttachmentInfo: storage.VolumeAttachmentInfo{
			DeviceName: "sdb",
		},
	}}
	inst := s.checkStartInstanceCustom(
		c, m, "pork", s.defaultConstraints,
		nil, nil, nil,
		expectedVolumes,
		expectedVolumeAttachments,
		nil, true,
	)

	// Cleanup.
	c.Assert(m.EnsureDead(), gc.IsNil)
	s.checkStopInstances(c, inst)
	s.waitForRemovalMark(c, m)
}

func (s *ProvisionerSuite) TestProvisioningDoesNotProvisionTheSameMachineAfterRestart(c *gc.C) {
	p := s.newEnvironProvisioner(c)
	defer workertest.CleanKill(c, p)

	// create a machine
	m, err := s.addMachine()
	c.Assert(err, jc.ErrorIsNil)
	s.checkStartInstance(c, m)

	// restart the PA
	workertest.CleanKill(c, p)
	p = s.newEnvironProvisioner(c)
	defer workertest.CleanKill(c, p)

	// check that there is only one machine provisioned.
	machines, err := s.State.AllMachines()
	c.Assert(err, jc.ErrorIsNil)
	c.Check(len(machines), gc.Equals, 2)
	c.Check(machines[0].Id(), gc.Equals, "0")
	c.Check(machines[1].CheckProvisioned("fake_nonce"), jc.IsFalse)

	// the PA should not create it a second time
	s.checkNoOperations(c)
}

func (s *ProvisionerSuite) TestDyingMachines(c *gc.C) {
	p := s.newEnvironProvisioner(c)
	defer workertest.CleanKill(c, p)

	// provision a machine
	m0, err := s.addMachine()
	c.Assert(err, jc.ErrorIsNil)
	s.checkStartInstance(c, m0)

	// stop the provisioner and make the machine dying
	workertest.CleanKill(c, p)
	err = m0.Destroy()
	c.Assert(err, jc.ErrorIsNil)

	// add a new, dying, unprovisioned machine
	m1, err := s.addMachine()
	c.Assert(err, jc.ErrorIsNil)
	err = m1.Destroy()
	c.Assert(err, jc.ErrorIsNil)

	// start the provisioner and wait for it to reap the useless machine
	p = s.newEnvironProvisioner(c)
	defer workertest.CleanKill(c, p)
	s.checkNoOperations(c)
	s.waitForRemovalMark(c, m1)

	// verify the other one's still fine
	err = m0.Refresh()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(m0.Life(), gc.Equals, state.Dying)
}

type mockTaskAPI struct {
	provisioner.TaskAPI
}

func (mock *mockTaskAPI) Machines(tags ...names.MachineTag) ([]apiprovisioner.MachineResult, error) {
	return nil, fmt.Errorf("error")
}

func (*mockTaskAPI) MachinesWithTransientErrors() ([]apiprovisioner.MachineStatusResult, error) {
	return nil, fmt.Errorf("error")
}

type mockDistributionGroupFinder struct {
	groups map[names.MachineTag][]string
}

func (mock *mockDistributionGroupFinder) DistributionGroupByMachineId(
	tags ...names.MachineTag,
) ([]apiprovisioner.DistributionGroupResult, error) {
	result := make([]apiprovisioner.DistributionGroupResult, len(tags))
	if len(mock.groups) == 0 {
		for i := range tags {
			result[i] = apiprovisioner.DistributionGroupResult{MachineIds: []string{}}
		}
	} else {
		for i, tag := range tags {
			if dg, ok := mock.groups[tag]; ok {
				result[i] = apiprovisioner.DistributionGroupResult{MachineIds: dg}
			} else {
				result[i] = apiprovisioner.DistributionGroupResult{
					MachineIds: []string{}, Err: &params.Error{Code: params.CodeNotFound, Message: "Fail"}}
			}
		}
	}
	return result, nil
}

func (s *ProvisionerSuite) TestMachineErrorsRetainInstances(c *gc.C) {
	task := s.newProvisionerTask(
		c,
		config.HarvestAll,
		s.Environ,
		s.provisioner,
		&mockDistributionGroupFinder{},
		mockToolsFinder{},
	)
	defer workertest.CleanKill(c, task)

	// create a machine
	m0, err := s.addMachine()
	c.Assert(err, jc.ErrorIsNil)
	s.checkStartInstance(c, m0)

	// create an instance out of band
	s.startUnknownInstance(c, "999")

	// start the provisioner and ensure it doesn't kill any
	// instances if there are errors getting machines.
	task = s.newProvisionerTask(
		c,
		config.HarvestAll,
		s.Environ,
		&mockTaskAPI{},
		&mockDistributionGroupFinder{},
		&mockToolsFinder{},
	)
	defer func() {
		err := worker.Stop(task)
		c.Assert(err, gc.ErrorMatches, ".*getting machine.*")
	}()
	s.checkNoOperations(c)
}

func (s *ProvisionerSuite) TestEnvironProvisionerObservesConfigChanges(c *gc.C) {
	p := s.newEnvironProvisioner(c)
	defer workertest.CleanKill(c, p)
	s.assertProvisionerObservesConfigChanges(c, p)
}

func (s *ProvisionerSuite) TestEnvironProvisionerObservesConfigChangesWorkerCount(c *gc.C) {
	p := s.newEnvironProvisioner(c)
	defer workertest.CleanKill(c, p)
	s.assertProvisionerObservesConfigChangesWorkerCount(c, p, false)
}

func (s *ProvisionerSuite) newProvisionerTask(
	c *gc.C,
	harvestingMethod config.HarvestMode,
	broker environs.InstanceBroker,
	taskAPI provisioner.TaskAPI,
	distributionGroupFinder provisioner.DistributionGroupFinder,
	toolsFinder provisioner.ToolsFinder,
) provisioner.ProvisionerTask {

	retryStrategy := provisioner.NewRetryStrategy(0*time.Second, 0)

	return s.newProvisionerTaskWithRetryStrategy(c, harvestingMethod, broker,
		taskAPI, distributionGroupFinder, toolsFinder, retryStrategy)
}

func (s *ProvisionerSuite) newProvisionerTaskWithRetryStrategy(
	c *gc.C,
	harvestingMethod config.HarvestMode,
	broker environs.InstanceBroker,
	taskAPI provisioner.TaskAPI,
	distributionGroupFinder provisioner.DistributionGroupFinder,
	toolsFinder provisioner.ToolsFinder,
	retryStrategy provisioner.RetryStrategy,
) provisioner.ProvisionerTask {

	machineWatcher, err := s.provisioner.WatchModelMachines()
	c.Assert(err, jc.ErrorIsNil)
	retryWatcher, err := s.provisioner.WatchMachineErrorRetry()
	c.Assert(err, jc.ErrorIsNil)
	auth, err := authentication.NewAPIAuthenticator(s.provisioner)
	c.Assert(err, jc.ErrorIsNil)

	w, err := provisioner.NewProvisionerTask(provisioner.TaskConfig{
		ControllerUUID:             s.ControllerConfig.ControllerUUID(),
		HostTag:                    names.NewMachineTag("0"),
		Logger:                     loggo.GetLogger("test"),
		HarvestMode:                harvestingMethod,
		TaskAPI:                    taskAPI,
		DistributionGroupFinder:    distributionGroupFinder,
		ToolsFinder:                toolsFinder,
		MachineWatcher:             machineWatcher,
		RetryWatcher:               retryWatcher,
		Broker:                     broker,
		Auth:                       auth,
		ImageStream:                imagemetadata.ReleasedStream,
		RetryStartInstanceStrategy: retryStrategy,
		CloudCallContextFunc:       func(_ stdcontext.Context) context.ProviderCallContext { return s.callCtx },
		NumProvisionWorkers:        numProvisionWorkersForTesting,
	})
	c.Assert(err, jc.ErrorIsNil)
	return w
}

func (s *ProvisionerSuite) TestHarvestNoneReapsNothing(c *gc.C) {

	task := s.newProvisionerTask(c, config.HarvestDestroyed, s.Environ, s.provisioner, &mockDistributionGroupFinder{}, mockToolsFinder{})
	defer workertest.CleanKill(c, task)
	task.SetHarvestMode(config.HarvestNone)

	// Create a machine and an unknown instance.
	m0, err := s.addMachine()
	c.Assert(err, jc.ErrorIsNil)
	s.checkStartInstance(c, m0)
	s.startUnknownInstance(c, "999")

	// Mark the first machine as dead.
	c.Assert(m0.EnsureDead(), gc.IsNil)

	// Ensure we're doing nothing.
	s.checkNoOperations(c)
}

func (s *ProvisionerSuite) TestHarvestUnknownReapsOnlyUnknown(c *gc.C) {
	task := s.newProvisionerTask(c,
		config.HarvestDestroyed,
		s.Environ,
		s.provisioner,
		&mockDistributionGroupFinder{},
		mockToolsFinder{},
	)
	defer workertest.CleanKill(c, task)
	task.SetHarvestMode(config.HarvestUnknown)

	// Create a machine and an unknown instance.
	m0, err := s.addMachine()
	c.Assert(err, jc.ErrorIsNil)
	i0 := s.checkStartInstance(c, m0)
	i1 := s.startUnknownInstance(c, "999")

	// Mark the first machine as dead.
	c.Assert(m0.EnsureDead(), gc.IsNil)

	// When only harvesting unknown machines, only one of the machines
	// is stopped.
	s.checkStopSomeInstances(c, []instances.Instance{i1}, []instances.Instance{i0})
	s.waitForRemovalMark(c, m0)
}

func (s *ProvisionerSuite) TestHarvestDestroyedReapsOnlyDestroyed(c *gc.C) {

	task := s.newProvisionerTask(
		c,
		config.HarvestDestroyed,
		s.Environ,
		s.provisioner,
		&mockDistributionGroupFinder{},
		mockToolsFinder{},
	)
	defer workertest.CleanKill(c, task)

	// Create a machine and an unknown instance.
	m0, err := s.addMachine()
	c.Assert(err, jc.ErrorIsNil)
	i0 := s.checkStartInstance(c, m0)
	i1 := s.startUnknownInstance(c, "999")

	// Mark the first machine as dead.
	c.Assert(m0.EnsureDead(), gc.IsNil)

	// When only harvesting destroyed machines, only one of the
	// machines is stopped.
	s.checkStopSomeInstances(c, []instances.Instance{i0}, []instances.Instance{i1})
	s.waitForRemovalMark(c, m0)
}

func (s *ProvisionerSuite) TestHarvestAllReapsAllTheThings(c *gc.C) {

	task := s.newProvisionerTask(c,
		config.HarvestDestroyed,
		s.Environ,
		s.provisioner,
		&mockDistributionGroupFinder{},
		mockToolsFinder{},
	)
	defer workertest.CleanKill(c, task)
	task.SetHarvestMode(config.HarvestAll)

	// Create a machine and an unknown instance.
	m0, err := s.addMachine()
	c.Assert(err, jc.ErrorIsNil)
	i0 := s.checkStartInstance(c, m0)
	i1 := s.startUnknownInstance(c, "999")

	// Mark the first machine as dead.
	c.Assert(m0.EnsureDead(), gc.IsNil)

	// Everything must die!
	s.checkStopSomeInstances(c, []instances.Instance{i0, i1}, []instances.Instance{})
	s.waitForRemovalMark(c, m0)
}

func (s *ProvisionerSuite) TestProvisionerObservesMachineJobs(c *gc.C) {
	s.PatchValue(&apiserverprovisioner.ErrorRetryWaitDelay, 5*time.Millisecond)
	broker := &mockBroker{Environ: s.Environ, retryCount: make(map[string]int),
		startInstanceFailureInfo: map[string]mockBrokerFailures{
			"3": {whenSucceed: 2, err: fmt.Errorf("error: some error")},
			"4": {whenSucceed: 2, err: fmt.Errorf("error: some error")},
		},
	}
	task := s.newProvisionerTask(c, config.HarvestAll, broker, s.provisioner, &mockDistributionGroupFinder{}, mockToolsFinder{})
	defer workertest.CleanKill(c, task)

	added := s.enableHA(c, 3)
	c.Assert(added, gc.HasLen, 2)
	s.checkStartInstances(c, added)
}

func assertAvailabilityZoneMachines(c *gc.C,
	machines []*state.Machine,
	failedAZMachines []*state.Machine,
	obtained []provisioner.AvailabilityZoneMachine,
) {
	if len(machines) > 0 {
		// Do machine zones match AvailabilityZoneMachine
		for _, m := range machines {
			zone, err := m.AvailabilityZone()
			c.Assert(err, jc.ErrorIsNil)
			found := 0
			for _, zoneInfo := range obtained {
				if zone == zoneInfo.ZoneName {
					c.Assert(zoneInfo.MachineIds.Contains(m.Id()), gc.Equals, true, gc.Commentf(
						"machine %q not found in list for zone %q; zone list: %#v", m.Id(), zone, zoneInfo,
					))
					found += 1
				}
			}
			c.Assert(found, gc.Equals, 1)
		}
	}
	if len(failedAZMachines) > 0 {
		for _, m := range failedAZMachines {
			// Is the failed machine listed as failed in at least one zone?
			failedZones := 0
			for _, zoneInfo := range obtained {
				if zoneInfo.FailedMachineIds.Contains(m.Id()) {
					failedZones += 1
				}
			}
			c.Assert(failedZones, jc.GreaterThan, 0)
		}
	}
}

// assertAvailabilityZoneMachinesDistribution checks to see if the
// machines have been distributed over the zones (with a maximum delta
// between the max and min number of machines of maxDelta). This check
// method works where there are no machine errors in the test case.
//
// Which machine will be in which zone is dependent on the order in
// which they are provisioned, therefore almost impossible to predict.
func assertAvailabilityZoneMachinesDistribution(c *gc.C, obtained []provisioner.AvailabilityZoneMachine, maxDelta int) {
	// Are the machines evenly distributed?  No zone should have
	// 2 machines more than any other zone.
	min, max := 1, 0
	counts := make(map[string]int)
	for _, zone := range obtained {
		count := zone.MachineIds.Size()
		counts[zone.ZoneName] = count
		if min > count {
			min = count
		}
		if max < count {
			max = count
		}
	}
	c.Assert(max-min, jc.LessThan, maxDelta+1, gc.Commentf("min = %d, max = %d, counts = %v", min, max, counts))
}

// checkAvailabilityZoneMachinesDistributionGroups checks to see if
// the distribution groups have been honored.
func checkAvailabilityZoneMachinesDistributionGroups(c *gc.C, groups map[names.MachineTag][]string, obtained []provisioner.AvailabilityZoneMachine) error {
	// The set containing the machines in a distribution group and the
	// machine whose distribution group this is, should not be in the
	// same AZ, unless there are more machines in the set, than AZs.
	// If there are more machines in the set than AZs, each AZ should have
	// the number of machines in the set divided by the number of AZ in it,
	// or 1 less than that number.
	//
	// e.g. if there are 5 machines in the set and 3 AZ, each AZ should have
	// 2 or 1 machines from the set in it.
	obtainedZoneCount := len(obtained)
	for tag, group := range groups {
		maxMachineInZoneCount := 1
		applicationMachinesCount := len(group) + 1
		if applicationMachinesCount > obtainedZoneCount {
			maxMachineInZoneCount = applicationMachinesCount / obtainedZoneCount
		}
		for _, z := range obtained {
			if z.MachineIds.Contains(tag.Id()) {
				intersection := z.MachineIds.Intersection(set.NewStrings(group...))
				machineCount := intersection.Size() + 1
				// For appropriate machine distribution, the number of machines in the
				// zone should be the same as maxMachineInZoneCount or 1 less.
				if machineCount == maxMachineInZoneCount || machineCount == maxMachineInZoneCount-1 {
					break
				}
				return errors.Errorf("%+v has too many of %s and %s", z.MachineIds, tag.Id(), group)
			}
		}
	}
	return nil
}

func (s *ProvisionerSuite) TestAvailabilityZoneMachinesStartMachines(c *gc.C) {
	// Per provider dummy, there will be 3 available availability zones.
	task := s.newProvisionerTask(c, config.HarvestDestroyed, s.Environ, s.provisioner, &mockDistributionGroupFinder{}, mockToolsFinder{})
	defer workertest.CleanKill(c, task)

	machines, err := s.addMachines(4)
	c.Assert(err, jc.ErrorIsNil)
	s.checkStartInstances(c, machines)

	availabilityZoneMachines := provisioner.GetCopyAvailabilityZoneMachines(task)
	assertAvailabilityZoneMachines(c, machines, nil, availabilityZoneMachines)
	assertAvailabilityZoneMachinesDistribution(c, availabilityZoneMachines, 1)
}

func (s *ProvisionerSuite) TestAvailabilityZoneMachinesStartMachinesAZFailures(c *gc.C) {
	// Per provider dummy, there will be 3 available availability zones.
	s.PatchValue(&apiserverprovisioner.ErrorRetryWaitDelay, 5*time.Millisecond)
	e := &mockBroker{
		Environ:    s.Environ,
		retryCount: make(map[string]int),
		startInstanceFailureInfo: map[string]mockBrokerFailures{
			"2": {whenSucceed: 1, err: errors.New("zing")},
		},
	}
	retryStrategy := provisioner.NewRetryStrategy(5*time.Millisecond, 2)
	task := s.newProvisionerTaskWithRetryStrategy(c, config.HarvestDestroyed,
		e, s.provisioner, &mockDistributionGroupFinder{}, mockToolsFinder{}, retryStrategy)
	defer workertest.CleanKill(c, task)

	machines, err := s.addMachines(4)
	c.Assert(err, jc.ErrorIsNil)
	s.checkStartInstances(c, machines)

	availabilityZoneMachines := provisioner.GetCopyAvailabilityZoneMachines(task)
	assertAvailabilityZoneMachines(c, machines, nil, availabilityZoneMachines)

	// The reason maxDelta is 2 here is because in certain failure cases this
	// may start two machines on each of two zones, and none on the other (if
	// the failing machine is started second or third, and the subsequent
	// machines are started before markMachineFailedInAZ() is called). See
	// https://github.com/juju/juju/pull/12267 for more detail.
	assertAvailabilityZoneMachinesDistribution(c, availabilityZoneMachines, 2)
}

func (s *ProvisionerSuite) TestAvailabilityZoneMachinesStartMachinesWithDG(c *gc.C) {
	// Per provider dummy, there will be 3 available availability zones.
	s.PatchValue(&apiserverprovisioner.ErrorRetryWaitDelay, 5*time.Millisecond)
	dgFinder := &mockDistributionGroupFinder{groups: map[names.MachineTag][]string{
		names.NewMachineTag("1"): {"3, 4"},
		names.NewMachineTag("2"): {},
		names.NewMachineTag("3"): {"1, 4"},
		names.NewMachineTag("4"): {"1, 3"},
		names.NewMachineTag("5"): {},
	}}

	task := s.newProvisionerTask(c, config.HarvestDestroyed, s.Environ, s.provisioner, dgFinder, mockToolsFinder{})
	defer workertest.CleanKill(c, task)

	machines, err := s.addMachines(5)
	c.Assert(err, jc.ErrorIsNil)
	s.checkStartInstances(c, machines)

	// 1, 2, 4 should be in different zones
	availabilityZoneMachines := provisioner.GetCopyAvailabilityZoneMachines(task)
	assertAvailabilityZoneMachines(c, machines, nil, availabilityZoneMachines)
	c.Assert(checkAvailabilityZoneMachinesDistributionGroups(c, dgFinder.groups, availabilityZoneMachines), jc.ErrorIsNil)
}

func (s *ProvisionerSuite) TestAvailabilityZoneMachinesStartMachinesAZFailuresWithDG(c *gc.C) {
	// Per provider dummy, there will be 3 available availability zones.
	s.PatchValue(&apiserverprovisioner.ErrorRetryWaitDelay, 5*time.Millisecond)
	e := &mockBroker{
		Environ:    s.Environ,
		retryCount: make(map[string]int),
		startInstanceFailureInfo: map[string]mockBrokerFailures{
			"2": {whenSucceed: 1, err: errors.New("zing")},
		},
	}
	dgFinder := &mockDistributionGroupFinder{groups: map[names.MachineTag][]string{
		names.NewMachineTag("1"): {"4", "5"},
		names.NewMachineTag("2"): {"3"},
		names.NewMachineTag("3"): {"2"},
		names.NewMachineTag("4"): {"1", "5"},
		names.NewMachineTag("5"): {"1", "4"},
	}}
	retryStrategy := provisioner.NewRetryStrategy(0*time.Second, 2)
	task := s.newProvisionerTaskWithRetryStrategy(c, config.HarvestDestroyed,
		e, s.provisioner, dgFinder, mockToolsFinder{}, retryStrategy)
	defer workertest.CleanKill(c, task)

	machines, err := s.addMachines(5)
	c.Assert(err, jc.ErrorIsNil)
	s.checkStartInstances(c, machines)

	availabilityZoneMachines := provisioner.GetCopyAvailabilityZoneMachines(task)
	assertAvailabilityZoneMachines(c, machines, []*state.Machine{machines[1]}, availabilityZoneMachines)
	c.Assert(checkAvailabilityZoneMachinesDistributionGroups(c, dgFinder.groups, availabilityZoneMachines), jc.ErrorIsNil)
}

func (s *ProvisionerSuite) TestProvisioningMachinesSingleMachineDGFailure(c *gc.C) {
	// If a single machine fails getting the distribution group,
	// ensure the other machines are still provisioned.
	dgFinder := &mockDistributionGroupFinder{
		groups: map[names.MachineTag][]string{
			names.NewMachineTag("2"): {"3", "5"},
			names.NewMachineTag("3"): {"2", "5"},
			names.NewMachineTag("4"): {"1"},
			names.NewMachineTag("5"): {"2", "3"},
		},
	}
	task := s.newProvisionerTask(c, config.HarvestDestroyed, s.Environ, s.provisioner, dgFinder, mockToolsFinder{})
	defer workertest.CleanKill(c, task)

	machines, err := s.addMachines(5)
	c.Assert(err, jc.ErrorIsNil)

	s.checkStartInstances(c, machines[1:])
	_, err = machines[0].InstanceId()
	c.Assert(err, jc.Satisfies, errors.IsNotProvisioned)

	availabilityZoneMachines := provisioner.GetCopyAvailabilityZoneMachines(task)
	assertAvailabilityZoneMachines(c, machines[1:], nil, availabilityZoneMachines)
	c.Assert(checkAvailabilityZoneMachinesDistributionGroups(c, dgFinder.groups, availabilityZoneMachines), jc.ErrorIsNil)
}

func (s *ProvisionerSuite) TestAvailabilityZoneMachinesStopMachines(c *gc.C) {
	// Per provider dummy, there will be 3 available availability zones.
	task := s.newProvisionerTask(
		c, config.HarvestDestroyed, s.Environ, s.provisioner, &mockDistributionGroupFinder{}, mockToolsFinder{})
	defer workertest.CleanKill(c, task)

	machines, err := s.addMachines(4)
	c.Assert(err, jc.ErrorIsNil)
	s.checkStartInstances(c, machines)

	availabilityZoneMachines := provisioner.GetCopyAvailabilityZoneMachines(task)
	assertAvailabilityZoneMachines(c, machines, nil, availabilityZoneMachines)
	assertAvailabilityZoneMachinesDistribution(c, availabilityZoneMachines, 1)

	c.Assert(machines[0].EnsureDead(), gc.IsNil)
	s.waitForRemovalMark(c, machines[0])

	assertAvailabilityZoneMachines(c, machines[1:], nil, provisioner.GetCopyAvailabilityZoneMachines(task))
}

func (s *ProvisionerSuite) TestProvisioningMachinesFailMachine(c *gc.C) {
	e := &mockBroker{
		Environ:    s.Environ,
		retryCount: make(map[string]int),
		startInstanceFailureInfo: map[string]mockBrokerFailures{
			"2": {whenSucceed: 2, err: errors.New("fail provisioning for TestAvailabilityZoneMachinesFailMachine")},
		},
	}
	task := s.newProvisionerTask(c, config.HarvestDestroyed,
		e, s.provisioner, &mockDistributionGroupFinder{}, mockToolsFinder{})
	defer workertest.CleanKill(c, task)

	machines, err := s.addMachines(4)
	c.Assert(err, jc.ErrorIsNil)
	mFail := machines[1]
	machines = append(machines[:1], machines[2:]...)
	s.checkStartInstances(c, machines)
	_, err = mFail.InstanceId()
	c.Assert(err, jc.Satisfies, errors.IsNotProvisioned)

	availabilityZoneMachines := provisioner.GetCopyAvailabilityZoneMachines(task)
	assertAvailabilityZoneMachines(c, machines, nil, availabilityZoneMachines)
	assertAvailabilityZoneMachinesDistribution(c, availabilityZoneMachines, 1)
}

func (s *ProvisionerSuite) TestAvailabilityZoneMachinesRestartTask(c *gc.C) {
	// Per provider dummy, there will be 3 available availability zones.
	task := s.newProvisionerTask(c, config.HarvestDestroyed, s.Environ, s.provisioner, &mockDistributionGroupFinder{}, mockToolsFinder{})
	defer workertest.CleanKill(c, task)

	machines, err := s.addMachines(4)
	c.Assert(err, jc.ErrorIsNil)
	s.checkStartInstances(c, machines)

	availabilityZoneMachinesBefore := provisioner.GetCopyAvailabilityZoneMachines(task)
	assertAvailabilityZoneMachines(c, machines, nil, availabilityZoneMachinesBefore)
	assertAvailabilityZoneMachinesDistribution(c, availabilityZoneMachinesBefore, 1)

	workertest.CleanKill(c, task)
	newTask := s.newProvisionerTask(c, config.HarvestDestroyed, s.Environ, s.provisioner, &mockDistributionGroupFinder{}, mockToolsFinder{})
	defer workertest.CleanKill(c, newTask)

	// Verify provisionerTask.availabilityZoneMachines is the same before and
	// after the provisionerTask is restarted.
	availabilityZoneMachinesAfter := provisioner.GetCopyAvailabilityZoneMachines(task)
	c.Assert(availabilityZoneMachinesBefore, jc.DeepEquals, availabilityZoneMachinesAfter)
}

func (s *ProvisionerSuite) TestProvisioningMachinesClearAZFailures(c *gc.C) {
	s.PatchValue(&apiserverprovisioner.ErrorRetryWaitDelay, 5*time.Millisecond)
	e := &mockBroker{
		Environ:    s.Environ,
		retryCount: make(map[string]int),
		startInstanceFailureInfo: map[string]mockBrokerFailures{
			"1": {whenSucceed: 3, err: errors.New("zing")},
		},
	}
	retryStrategy := provisioner.NewRetryStrategy(5*time.Millisecond, 4)
	task := s.newProvisionerTaskWithRetryStrategy(c, config.HarvestDestroyed,
		e, s.provisioner, &mockDistributionGroupFinder{}, mockToolsFinder{}, retryStrategy)
	defer workertest.CleanKill(c, task)

	machine, err := s.addMachine()
	c.Assert(err, jc.ErrorIsNil)
	s.checkStartInstance(c, machine)
	count := e.getRetryCount(machine.Id())
	c.Assert(count, gc.Equals, 3)
	machineAZ, err := machine.AvailabilityZone()
	c.Assert(err, jc.ErrorIsNil)
	// Zones 3 and 4 have the same machine count, one is picked at random.
	c.Assert(set.NewStrings("zone3", "zone4").Contains(machineAZ), jc.IsTrue)
}

func (s *ProvisionerSuite) TestProvisioningMachinesDerivedAZ(c *gc.C) {
	s.PatchValue(&apiserverprovisioner.ErrorRetryWaitDelay, 5*time.Millisecond)
	e := &mockBroker{
		Environ:    s.Environ,
		retryCount: make(map[string]int),
		startInstanceFailureInfo: map[string]mockBrokerFailures{
			"2": {whenSucceed: 3, err: errors.New("zing")},
			"3": {whenSucceed: 1, err: errors.New("zing")},
			"5": {whenSucceed: 1, err: environs.ZoneIndependentError(errors.New("arf"))},
		},
		derivedAZ: map[string][]string{
			"1": {"fail-zone"},
			"2": {"zone4"},
			"3": {"zone1", "zone4"},
			"4": {"zone1"},
			"5": {"zone3"},
		},
	}
	retryStrategy := provisioner.NewRetryStrategy(5*time.Millisecond, 2)
	task := s.newProvisionerTaskWithRetryStrategy(c, config.HarvestDestroyed,
		e, s.provisioner, &mockDistributionGroupFinder{}, mockToolsFinder{}, retryStrategy)
	defer workertest.CleanKill(c, task)

	machines, err := s.addMachines(5)
	c.Assert(err, jc.ErrorIsNil)
	mFail := machines[:2]
	mSucceed := machines[2:]

	s.checkStartInstances(c, mSucceed)
	c.Assert(e.getRetryCount(mSucceed[0].Id()), gc.Equals, 1)
	c.Assert(e.getRetryCount(mSucceed[2].Id()), gc.Equals, 1)

	// This synchronisation addresses a potential race condition.
	// It can happen that upon successful return from checkStartInstances
	// The machine(s) arranged for provisioning failure have not yet been
	// retried the specified number of times; so we wait.
	id := mFail[1].Id()
	timeout := time.After(coretesting.LongWait)
	for e.getRetryCount(id) < 3 {
		select {
		case <-timeout:
			c.Fatalf("Failed provision of %q did not retry 3 times", id)
		default:
		}
	}

	_, err = mFail[0].InstanceId()
	c.Assert(err, jc.Satisfies, errors.IsNotProvisioned)
	_, err = mFail[1].InstanceId()
	c.Assert(err, jc.Satisfies, errors.IsNotProvisioned)

	availabilityZoneMachines := provisioner.GetCopyAvailabilityZoneMachines(task)
	assertAvailabilityZoneMachines(c, mSucceed, nil, availabilityZoneMachines)

	for i, zone := range []string{"zone1", "zone3"} {
		machineAZ, err := mSucceed[i+1].AvailabilityZone()
		c.Assert(err, jc.ErrorIsNil)
		c.Assert(machineAZ, gc.Equals, zone)
	}
}

func (s *ProvisionerSuite) TestProvisioningMachinesNoZonedEnviron(c *gc.C) {
	// Make sure the provisioner still works for providers which do not
	// implement the ZonedEnviron interface.
	noZonedEnvironBroker := &mockNoZonedEnvironBroker{Environ: s.Environ}
	task := s.newProvisionerTask(c,
		config.HarvestDestroyed,
		noZonedEnvironBroker,
		s.provisioner,
		&mockDistributionGroupFinder{},
		mockToolsFinder{})
	defer workertest.CleanKill(c, task)

	machines, err := s.addMachines(4)
	c.Assert(err, jc.ErrorIsNil)
	s.checkStartInstances(c, machines)

	expected := provisioner.GetCopyAvailabilityZoneMachines(task)
	c.Assert(expected, gc.HasLen, 0)
}

type mockNoZonedEnvironBroker struct {
	environs.Environ
}

func (b *mockNoZonedEnvironBroker) StartInstance(ctx context.ProviderCallContext, args environs.StartInstanceParams) (*environs.StartInstanceResult, error) {
	return b.Environ.StartInstance(ctx, args)
}

type mockBroker struct {
	environs.Environ

	mu                       sync.Mutex
	retryCount               map[string]int
	startInstanceFailureInfo map[string]mockBrokerFailures
	derivedAZ                map[string][]string
}

type mockBrokerFailures struct {
	err         error
	whenSucceed int
}

func (b *mockBroker) StartInstance(ctx context.ProviderCallContext, args environs.StartInstanceParams) (*environs.StartInstanceResult, error) {
	// All machines are provisioned successfully the first time unless
	// mock.startInstanceFailureInfo is configured.
	//
	id := args.InstanceConfig.MachineId
	b.mu.Lock()
	defer b.mu.Unlock()
	retries := b.retryCount[id]
	whenSucceed := 0
	var returnError error
	if failureInfo, ok := b.startInstanceFailureInfo[id]; ok {
		whenSucceed = failureInfo.whenSucceed
		returnError = failureInfo.err
	}
	if retries == whenSucceed {
		return b.Environ.StartInstance(ctx, args)
	} else {
		b.retryCount[id] = retries + 1
	}
	return nil, returnError
}

func (b *mockBroker) getRetryCount(id string) int {
	b.mu.Lock()
	retries := b.retryCount[id]
	b.mu.Unlock()
	return retries
}

// ZonedEnviron necessary for provisionerTask.populateAvailabilityZoneMachines where
// mockBroker used.

func (b *mockBroker) AvailabilityZones(ctx context.ProviderCallContext) (corenetwork.AvailabilityZones, error) {
	return b.Environ.(providercommon.ZonedEnviron).AvailabilityZones(ctx)
}

func (b *mockBroker) InstanceAvailabilityZoneNames(ctx context.ProviderCallContext, ids []instance.Id) (map[instance.Id]string, error) {
	return b.Environ.(providercommon.ZonedEnviron).InstanceAvailabilityZoneNames(ctx, ids)
}

func (b *mockBroker) DeriveAvailabilityZones(ctx context.ProviderCallContext, args environs.StartInstanceParams) ([]string, error) {
	id := args.InstanceConfig.MachineId
	b.mu.Lock()
	defer b.mu.Unlock()
	if derivedAZ, ok := b.derivedAZ[id]; ok {
		return derivedAZ, nil
	}
	return b.Environ.(providercommon.ZonedEnviron).DeriveAvailabilityZones(ctx, args)
}

type mockToolsFinder struct {
}

func (f mockToolsFinder) FindTools(number version.Number, os string, a string) (coretools.List, error) {
	v, err := version.ParseBinary(fmt.Sprintf("%s-%s-%s", number, os, arch.HostArch()))
	if err != nil {
		return nil, err
	}
	if a == "" {
		return nil, errors.New("missing arch")
	}
	v.Arch = a
	return coretools.List{&coretools.Tools{Version: v}}, nil
}
