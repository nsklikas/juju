// Copyright 2012-2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package context_test

import (
	"sort"

	"github.com/juju/names/v5"
	"github.com/juju/proxy"
	jc "github.com/juju/testing/checkers"
	"github.com/juju/utils/v3/keyvalues"
	"github.com/juju/version/v2"
	"go.uber.org/mock/gomock"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/core/base"
	jujuos "github.com/juju/juju/core/os"
	"github.com/juju/juju/core/os/ostype"
	"github.com/juju/juju/core/secrets"
	"github.com/juju/juju/internal/worker/uniter/runner/context"
	"github.com/juju/juju/internal/worker/uniter/runner/context/mocks"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/juju/testing"
	jujuversion "github.com/juju/juju/version"
)

type EnvSuite struct {
	testing.BaseSuite
}

var _ = gc.Suite(&EnvSuite{})

func (s *EnvSuite) assertVars(c *gc.C, actual []string, expect ...[]string) {
	var fullExpect []string
	for _, someExpect := range expect {
		fullExpect = append(fullExpect, someExpect...)
	}
	sort.Strings(actual)
	sort.Strings(fullExpect)
	c.Assert(actual, jc.DeepEquals, fullExpect)
}

func (s *EnvSuite) getPaths() (paths context.Paths, expectVars []string) {
	// note: path-munging is os-dependent, not included in expectVars
	return MockEnvPaths{}, []string{
		"CHARM_DIR=path-to-charm",
		"JUJU_CHARM_DIR=path-to-charm",
		"JUJU_AGENT_SOCKET_ADDRESS=path-to-jujuc.socket",
		"JUJU_AGENT_SOCKET_NETWORK=unix",
	}
}

func (s *EnvSuite) getContext(newProxyOnly bool, state context.State, unit context.HookUnit) (ctx *context.HookContext, expectVars []string) {
	var (
		legacyProxy proxy.Settings
		jujuProxy   proxy.Settings
		proxy       = proxy.Settings{
			Http:    "some-http-proxy",
			Https:   "some-https-proxy",
			Ftp:     "some-ftp-proxy",
			NoProxy: "some-no-proxy",
		}
	)
	if newProxyOnly {
		jujuProxy = proxy
	} else {
		legacyProxy = proxy
	}

	expected := []string{
		"JUJU_CONTEXT_ID=some-context-id",
		"JUJU_HOOK_NAME=some-hook-name",
		"JUJU_MODEL_UUID=model-uuid-deadbeef",
		"JUJU_PRINCIPAL_UNIT=this-unit/123",
		"JUJU_MODEL_NAME=some-model-name",
		"JUJU_UNIT_NAME=this-unit/123",
		"JUJU_METER_STATUS=PURPLE",
		"JUJU_METER_INFO=proceed with care",
		"JUJU_SLA=essential",
		"JUJU_API_ADDRESSES=he.re:12345 the.re:23456",
		"JUJU_MACHINE_ID=42",
		"JUJU_AVAILABILITY_ZONE=some-zone",
		"JUJU_VERSION=1.2.3",
		"CLOUD_API_VERSION=6.66",
	}
	if newProxyOnly {
		expected = append(expected,
			"JUJU_CHARM_HTTP_PROXY=some-http-proxy",
			"JUJU_CHARM_HTTPS_PROXY=some-https-proxy",
			"JUJU_CHARM_FTP_PROXY=some-ftp-proxy",
			"JUJU_CHARM_NO_PROXY=some-no-proxy",
		)
	} else {
		expected = append(expected,
			"http_proxy=some-http-proxy",
			"HTTP_PROXY=some-http-proxy",
			"https_proxy=some-https-proxy",
			"HTTPS_PROXY=some-https-proxy",
			"ftp_proxy=some-ftp-proxy",
			"FTP_PROXY=some-ftp-proxy",
			"no_proxy=some-no-proxy",
			"NO_PROXY=some-no-proxy",
			// JUJU_CHARM prefixed proxy values are always specified
			// even if empty.
			"JUJU_CHARM_HTTP_PROXY=",
			"JUJU_CHARM_HTTPS_PROXY=",
			"JUJU_CHARM_FTP_PROXY=",
			"JUJU_CHARM_NO_PROXY=",
		)
	}
	// It doesn't make sense that we set both legacy and juju proxy
	// settings, but by setting both to different values, we can see
	// what the environment values are.
	return context.NewModelHookContext(context.ModelHookContextParams{
		ID:                  "some-context-id",
		HookName:            "some-hook-name",
		ModelUUID:           "model-uuid-deadbeef",
		ModelName:           "some-model-name",
		UnitName:            "this-unit/123",
		MeterCode:           "PURPLE",
		MeterInfo:           "proceed with care",
		SLALevel:            "essential",
		AvailZone:           "some-zone",
		APIAddresses:        []string{"he.re:12345", "the.re:23456"},
		LegacyProxySettings: legacyProxy,
		JujuProxySettings:   jujuProxy,
		MachineTag:          names.NewMachineTag("42"),
		State:               state,
		Unit:                unit,
	}), expected
}

func (s *EnvSuite) setSecret(ctx *context.HookContext) (expectVars []string) {
	url := secrets.NewURI()
	context.SetEnvironmentHookContextSecret(ctx, url.String(), nil, nil, nil)
	return []string{
		"JUJU_SECRET_ID=" + url.String(),
		"JUJU_SECRET_LABEL=label-" + url.String(),
		"JUJU_SECRET_REVISION=666",
	}
}

func (s *EnvSuite) setWorkload(ctx *context.HookContext) (expectVars []string) {
	workload := "wrk"
	context.SetEnvironmentHookContextWorkload(ctx, workload)
	return []string{
		"JUJU_WORKLOAD_NAME=" + workload,
	}
}

func (s *EnvSuite) setNotice(ctx *context.HookContext) (expectVars []string) {
	id := "1"
	typ := "custom"
	key := "a.com/b"
	context.SetEnvironmentHookContextNotice(ctx, id, typ, key)
	return []string{
		"JUJU_NOTICE_ID=" + id,
		"JUJU_NOTICE_TYPE=" + typ,
		"JUJU_NOTICE_KEY=" + key,
	}
}

// setCheck sets the context for a check hook.
func (s *EnvSuite) setCheck(ctx *context.HookContext) (expectVars []string) {
	name := "http-check"
	context.SetEnvironmentHookContextCheck(ctx, name)
	return []string{
		"JUJU_PEBBLE_CHECK_NAME=" + name,
	}
}

func (s *EnvSuite) setRelation(ctx *context.HookContext) (expectVars []string) {
	context.SetEnvironmentHookContextRelation(ctx, 22, "an-endpoint", "that-unit/456", "that-app", "")
	return []string{
		"JUJU_RELATION=an-endpoint",
		"JUJU_RELATION_ID=an-endpoint:22",
		"JUJU_REMOTE_UNIT=that-unit/456",
		"JUJU_REMOTE_APP=that-app",
	}
}

func (s *EnvSuite) setDepartingRelation(ctx *context.HookContext) (expectVars []string) {
	context.SetEnvironmentHookContextRelation(ctx, 22, "an-endpoint", "that-unit/456", "that-app", "that-unit/456")
	return []string{
		"JUJU_RELATION=an-endpoint",
		"JUJU_RELATION_ID=an-endpoint:22",
		"JUJU_REMOTE_UNIT=that-unit/456",
		"JUJU_REMOTE_APP=that-app",
		"JUJU_DEPARTING_UNIT=that-unit/456",
	}
}

func (s *EnvSuite) setStorage(ctx *context.HookContext) (expectVars []string) {
	tag := names.NewStorageTag("data/0")
	context.SetEnvironmentHookContextStorage(ctx, tag)
	return []string{
		"JUJU_STORAGE_ID=data/0",
		"JUJU_STORAGE_KIND=block",
		"JUJU_STORAGE_LOCATION=/dev/sdb",
	}
}

func (s *EnvSuite) TestEnvSetsPath(c *gc.C) {
	paths := context.OSDependentEnvVars(MockEnvPaths{}, context.NewHostEnvironmenter())
	c.Assert(paths, gc.Not(gc.HasLen), 0)
	vars, err := keyvalues.Parse(paths, true)
	c.Assert(err, jc.ErrorIsNil)
	key := "PATH"
	c.Assert(vars[key], gc.Not(gc.Equals), "")
}

func (s *EnvSuite) TestEnvUbuntu(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	state := mocks.NewMockState(ctrl)
	state.EXPECT().StorageAttachment(names.NewStorageTag("data/0"), names.NewUnitTag("this-unit/123")).Return(params.StorageAttachment{
		Kind:     params.StorageKindBlock,
		Location: "/dev/sdb",
	}, nil).AnyTimes()
	unit := mocks.NewMockHookUnit(ctrl)
	unit.EXPECT().Tag().Return(names.NewUnitTag("this-unit/123")).AnyTimes()

	s.PatchValue(&jujuos.HostOS, func() ostype.OSType { return ostype.Ubuntu })
	s.PatchValue(&jujuversion.Current, version.MustParse("1.2.3"))

	ubuntuVars := []string{
		"APT_LISTCHANGES_FRONTEND=none",
		"DEBIAN_FRONTEND=noninteractive",
		"LANG=C.UTF-8",
		"TERM=tmux-256color",
		"PATH=path-to-tools:foo:bar",
	}

	environmenter := context.NewRemoteEnvironmenter(
		func() []string { return []string{} },
		func(k string) string {
			switch k {
			case "PATH":
				return "foo:bar"
			}
			return ""
		},
		func(k string) (string, bool) {
			switch k {
			case "PATH":
				return "foo:bar", true
			}
			return "", false
		},
	)

	ctx, contextVars := s.getContext(false, state, unit)
	paths, pathsVars := s.getPaths()
	actualVars, err := ctx.HookVars(paths, false, environmenter)
	c.Assert(err, jc.ErrorIsNil)
	s.assertVars(c, actualVars, contextVars, pathsVars, ubuntuVars)

	relationVars := s.setDepartingRelation(ctx)
	secretVars := s.setSecret(ctx)
	storageVars := s.setStorage(ctx)
	workloadVars := s.setWorkload(ctx)
	noticeVars := s.setNotice(ctx)
	checkVars := s.setCheck(ctx)
	actualVars, err = ctx.HookVars(paths, false, environmenter)
	c.Assert(err, jc.ErrorIsNil)
	s.assertVars(c, actualVars, contextVars, pathsVars, ubuntuVars, relationVars, secretVars, storageVars, workloadVars, noticeVars, checkVars)
}

func (s *EnvSuite) TestEnvCentos(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	state := mocks.NewMockState(ctrl)
	state.EXPECT().StorageAttachment(names.NewStorageTag("data/0"), names.NewUnitTag("this-unit/123")).Return(params.StorageAttachment{
		Kind:     params.StorageKindBlock,
		Location: "/dev/sdb",
	}, nil).AnyTimes()
	unit := mocks.NewMockHookUnit(ctrl)
	unit.EXPECT().Tag().Return(names.NewUnitTag("this-unit/123")).AnyTimes()

	s.PatchValue(&jujuos.HostOS, func() ostype.OSType { return ostype.CentOS })
	s.PatchValue(&jujuversion.Current, version.MustParse("1.2.3"))

	// TERM is different for centos7.
	for _, testBase := range []base.Base{base.MustParseBaseFromString("centos@7"), base.MustParseBaseFromString("centos@8")} {
		s.PatchValue(&jujuos.HostBase, func() (base.Base, error) { return testBase, nil })
		centosVars := []string{
			"LANG=C.UTF-8",
			"PATH=path-to-tools:foo:bar",
		}

		if testBase.Channel.Track == "7" {
			centosVars = append(centosVars, "TERM=screen-256color")
		} else {
			centosVars = append(centosVars, "TERM=tmux-256color")
		}

		environmenter := context.NewRemoteEnvironmenter(
			func() []string { return []string{} },
			func(k string) string {
				switch k {
				case "PATH":
					return "foo:bar"
				}
				return ""
			},
			func(k string) (string, bool) {
				switch k {
				case "PATH":
					return "foo:bar", true
				}
				return "", false
			},
		)

		ctx, contextVars := s.getContext(false, state, unit)
		paths, pathsVars := s.getPaths()
		actualVars, err := ctx.HookVars(paths, false, environmenter)
		c.Assert(err, jc.ErrorIsNil)
		s.assertVars(c, actualVars, contextVars, pathsVars, centosVars)

		relationVars := s.setRelation(ctx)
		secretVars := s.setSecret(ctx)
		actualVars, err = ctx.HookVars(paths, false, environmenter)
		c.Assert(err, jc.ErrorIsNil)
		s.assertVars(c, actualVars, contextVars, pathsVars, centosVars, relationVars, secretVars)
	}
}

func (s *EnvSuite) TestEnvGenericLinux(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	state := mocks.NewMockState(ctrl)
	state.EXPECT().StorageAttachment(names.NewStorageTag("data/0"), names.NewUnitTag("this-unit/123")).Return(params.StorageAttachment{
		Kind:     params.StorageKindBlock,
		Location: "/dev/sdb",
	}, nil).AnyTimes()
	unit := mocks.NewMockHookUnit(ctrl)
	unit.EXPECT().Tag().Return(names.NewUnitTag("this-unit/123")).AnyTimes()

	s.PatchValue(&jujuos.HostOS, func() ostype.OSType { return ostype.GenericLinux })
	s.PatchValue(&jujuversion.Current, version.MustParse("1.2.3"))

	genericLinuxVars := []string{
		"LANG=C.UTF-8",
		"PATH=path-to-tools:foo:bar",
		"TERM=screen",
	}

	environmenter := context.NewRemoteEnvironmenter(
		func() []string { return []string{} },
		func(k string) string {
			switch k {
			case "PATH":
				return "foo:bar"
			}
			return ""
		},
		func(k string) (string, bool) {
			switch k {
			case "PATH":
				return "foo:bar", true
			}
			return "", false
		},
	)

	ctx, contextVars := s.getContext(false, state, unit)
	paths, pathsVars := s.getPaths()
	actualVars, err := ctx.HookVars(paths, false, environmenter)
	c.Assert(err, jc.ErrorIsNil)
	s.assertVars(c, actualVars, contextVars, pathsVars, genericLinuxVars)

	relationVars := s.setRelation(ctx)
	secretVars := s.setSecret(ctx)
	actualVars, err = ctx.HookVars(paths, false, environmenter)
	c.Assert(err, jc.ErrorIsNil)
	s.assertVars(c, actualVars, contextVars, pathsVars, genericLinuxVars, relationVars, secretVars)
}

func (s *EnvSuite) TestHostEnv(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	state := mocks.NewMockState(ctrl)
	state.EXPECT().StorageAttachment(names.NewStorageTag("data/0"), names.NewUnitTag("this-unit/123")).Return(params.StorageAttachment{
		Kind:     params.StorageKindBlock,
		Location: "/dev/sdb",
	}, nil).AnyTimes()
	unit := mocks.NewMockHookUnit(ctrl)
	unit.EXPECT().Tag().Return(names.NewUnitTag("this-unit/123")).AnyTimes()

	s.PatchValue(&jujuos.HostOS, func() ostype.OSType { return ostype.GenericLinux })
	s.PatchValue(&jujuversion.Current, version.MustParse("1.2.3"))

	genericLinuxVars := []string{
		"LANG=C.UTF-8",
		"PATH=path-to-tools:foo:bar",
		"TERM=screen",
	}

	environmenter := context.NewRemoteEnvironmenter(
		func() []string { return []string{"KUBERNETES_SERVICE=test"} },
		func(k string) string {
			switch k {
			case "PATH":
				return "foo:bar"
			}
			return ""
		},
		func(k string) (string, bool) {
			switch k {
			case "KUBERNETES_SERVICE":
				return "test", true
			case "PATH":
				return "foo:bar", true
			}
			return "", false
		},
	)

	ctx, contextVars := s.getContext(false, state, unit)
	paths, pathsVars := s.getPaths()
	actualVars, err := ctx.HookVars(paths, false, environmenter)
	c.Assert(err, jc.ErrorIsNil)
	s.assertVars(c, actualVars, contextVars, pathsVars, genericLinuxVars, []string{"KUBERNETES_SERVICE=test"})

	relationVars := s.setRelation(ctx)
	secretVars := s.setSecret(ctx)
	actualVars, err = ctx.HookVars(paths, false, environmenter)
	c.Assert(err, jc.ErrorIsNil)
	s.assertVars(c, actualVars, contextVars, pathsVars, genericLinuxVars, relationVars, secretVars, []string{"KUBERNETES_SERVICE=test"})
}

func (s *EnvSuite) TestContextDependentDoesNotIncludeUnSet(c *gc.C) {
	environmenter := context.NewRemoteEnvironmenter(
		func() []string { return []string{} },
		func(_ string) string { return "" },
		func(_ string) (string, bool) { return "", false },
	)

	c.Assert(len(context.ContextDependentEnvVars(environmenter)), gc.Equals, 0)
}

func (s *EnvSuite) TestContextDependentDoesIncludeAll(c *gc.C) {
	counter := 0
	environmenter := context.NewRemoteEnvironmenter(
		func() []string { return []string{} },
		func(_ string) string { return "" },
		func(_ string) (string, bool) {
			counter = counter + 1
			return "dummy-val", true
		},
	)
	c.Assert(len(context.ContextDependentEnvVars(environmenter)), gc.Equals, counter)
}

func (s *EnvSuite) TestContextDependentPartialInclude(c *gc.C) {
	counter := 0
	environmenter := context.NewRemoteEnvironmenter(
		func() []string { return []string{} },
		func(_ string) string { return "" },
		func(_ string) (string, bool) {
			// We are just going to include the first two env vars here to make
			// sure that both true and false statements work
			if counter < 2 {
				counter = counter + 1
				return "dummy-val", true
			}
			return "", false
		},
	)

	c.Assert(len(context.ContextDependentEnvVars(environmenter)), gc.Equals, counter)
	c.Assert(counter, gc.Equals, 2)
}

func (s *EnvSuite) TestContextDependentCallsAllVarKeys(c *gc.C) {
	queriedVars := map[string]bool{}
	environmenter := context.NewRemoteEnvironmenter(
		func() []string { return []string{} },
		func(_ string) string { return "" },
		func(k string) (string, bool) {
			for _, envKey := range context.ContextAllowedEnvVars {
				if envKey == k && queriedVars[k] == false {
					queriedVars[k] = true
					return "dummy-val", true
				} else if envKey == k && queriedVars[envKey] == true {
					c.Errorf("key %q has been queried more than once", k)
					return "", false
				}
			}
			c.Errorf("unexpected key %q has been queried for", k)
			return "", false
		},
	)

	rval := context.ContextDependentEnvVars(environmenter)
	c.Assert(len(rval), gc.Equals, len(queriedVars))
	c.Assert(len(queriedVars), gc.Equals, len(context.ContextAllowedEnvVars))
}
