// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package context

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/juju/charm/v12"
	"github.com/juju/charm/v12/hooks"
	"github.com/juju/errors"
	"github.com/juju/loggo"
	"github.com/juju/names/v5"

	"github.com/juju/juju/api/agent/uniter"
	"github.com/juju/juju/core/leadership"
	"github.com/juju/juju/core/life"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/core/network"
	"github.com/juju/juju/core/secrets"
	"github.com/juju/juju/internal/worker/uniter/hook"
	"github.com/juju/juju/internal/worker/uniter/runner/context/payloads"
	"github.com/juju/juju/internal/worker/uniter/runner/context/resources"
	"github.com/juju/juju/internal/worker/uniter/runner/jujuc"
	"github.com/juju/juju/rpc/params"
	jujusecrets "github.com/juju/juju/secrets"
)

// CommandInfo specifies the information necessary to run a command.
type CommandInfo struct {
	// RelationId is the relation context to execute the commands in.
	RelationId int
	// RemoteUnitName is the remote unit for the relation context.
	RemoteUnitName string
	// TODO(jam): 2019-10-23 Add RemoteApplicationName
	// ForceRemoteUnit skips unit inference and existence validation.
	ForceRemoteUnit bool
}

// ContextFactory represents a long-lived object that can create execution contexts
// relevant to a specific unit.
type ContextFactory interface {
	// CommandContext creates a new context for running a juju command.
	CommandContext(commandInfo CommandInfo) (*HookContext, error)

	// HookContext creates a new context for running a juju hook.
	HookContext(hookInfo hook.Info) (*HookContext, error)

	// ActionContext creates a new context for running a juju action.
	ActionContext(actionData *ActionData) (*HookContext, error)
}

// StorageContextAccessor is an interface providing access to StorageContexts
// for a hooks.Context.
type StorageContextAccessor interface {

	// StorageTags returns the tags of storage instances attached to
	// the unit.
	StorageTags() ([]names.StorageTag, error)

	// Storage returns the hooks.ContextStorageAttachment with the
	// supplied tag if it was found, and whether it was found.
	Storage(names.StorageTag) (jujuc.ContextStorageAttachment, error)
}

// SecretsAccessor is used by the hook context to access the secrets backend.
type SecretsAccessor interface {
	// CreateSecretURIs is used by secret-add to get URIs
	// for added secrets.
	CreateSecretURIs(int) ([]*secrets.URI, error)

	// SecretMetadata is used by secrets-get to fetch
	// metadata for secrets.
	SecretMetadata() ([]secrets.SecretOwnerMetadata, error)

	// SecretRotated records the outcome of rotating a secret.
	SecretRotated(uri string, oldRevision int) error
}

// SecretsBackendGetter creates a secrets backend client.
type SecretsBackendGetter func() (jujusecrets.BackendsClient, error)

// RelationsFunc is used to get snapshots of relation membership at context
// creation time.
type RelationsFunc func() map[int]*RelationInfo

type contextFactory struct {
	// API connection fields; unit should be deprecated, but isn't yet.
	unit                 *uniter.Unit
	state                *uniter.State
	resources            *uniter.ResourcesFacadeClient
	payloads             *uniter.PayloadFacadeClient
	secretsClient        SecretsAccessor
	secretsBackendGetter SecretsBackendGetter
	tracker              leadership.Tracker

	logger loggo.Logger

	// Fields that shouldn't change in a factory's lifetime.
	paths      Paths
	modelUUID  string
	modelName  string
	modelType  model.ModelType
	machineTag names.MachineTag
	clock      Clock
	zone       string
	principal  string

	// Callback to get relation state snapshot.
	getRelationInfos RelationsFunc
	relationCaches   map[int]*RelationCache
}

// FactoryConfig contains configuration values
// for the context factory.
type FactoryConfig struct {
	State                *uniter.State
	SecretsClient        SecretsAccessor
	SecretsBackendGetter SecretsBackendGetter
	Unit                 *uniter.Unit
	Resources            *uniter.ResourcesFacadeClient
	Payloads             *uniter.PayloadFacadeClient
	Tracker              leadership.Tracker
	GetRelationInfos     RelationsFunc
	Paths                Paths
	Clock                Clock
	Logger               loggo.Logger
}

// NewContextFactory returns a ContextFactory capable of creating execution contexts backed
// by the supplied unit's supplied API connection.
func NewContextFactory(config FactoryConfig) (ContextFactory, error) {
	m, err := config.State.Model()
	if err != nil {
		return nil, errors.Trace(err)
	}
	var (
		machineTag names.MachineTag
		zone       string
	)
	if m.ModelType == model.IAAS {
		machineTag, err = config.Unit.AssignedMachine()
		if err != nil {
			return nil, errors.Trace(err)
		}

		zone, err = config.Unit.AvailabilityZone()
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	principal, ok, err := config.Unit.PrincipalName()
	if err != nil {
		return nil, errors.Trace(err)
	} else if !ok {
		principal = ""
	}

	f := &contextFactory{
		unit:                 config.Unit,
		state:                config.State,
		resources:            config.Resources,
		payloads:             config.Payloads,
		secretsClient:        config.SecretsClient,
		secretsBackendGetter: config.SecretsBackendGetter,
		tracker:              config.Tracker,
		logger:               config.Logger,
		paths:                config.Paths,
		modelUUID:            m.UUID,
		modelName:            m.Name,
		machineTag:           machineTag,
		getRelationInfos:     config.GetRelationInfos,
		relationCaches:       map[int]*RelationCache{},
		clock:                config.Clock,
		zone:                 zone,
		principal:            principal,
		modelType:            m.ModelType,
	}
	return f, nil
}

// newId returns a probably-unique identifier for a new context, containing the
// supplied string.
func (f *contextFactory) newId(name string) (string, error) {
	randomData := [16]byte{}
	_, err := rand.Read(randomData[:])
	if err != nil {
		return "", fmt.Errorf("cannot generate id for hook context: %w", err)
	}
	randomComponent := hex.EncodeToString(randomData[:])
	return fmt.Sprintf("%s-%s-%s", f.unit.Name(), name, randomComponent), nil
}

// coreContext creates a new context with all unspecialised fields filled in.
func (f *contextFactory) coreContext() (*HookContext, error) {
	leadershipContext := newLeadershipContext(
		f.state.LeadershipSettings,
		f.tracker,
		f.unit.Name(),
	)
	ctx := &HookContext{
		unit:                 f.unit,
		state:                f.state,
		secretsClient:        f.secretsClient,
		secretsBackendGetter: f.secretsBackendGetter,
		LeadershipContext:    leadershipContext,
		uuid:                 f.modelUUID,
		modelName:            f.modelName,
		modelType:            f.modelType,
		unitName:             f.unit.Name(),
		assignedMachineTag:   f.machineTag,
		relations:            f.getContextRelations(),
		relationId:           -1,
		clock:                f.clock,
		logger:               f.logger,
		availabilityZone:     f.zone,
		principal:            f.principal,
		ResourcesHookContext: &resources.ResourcesHookContext{
			Client:       f.resources,
			ResourcesDir: f.paths.GetResourcesDir(),
			Logger:       f.logger,
		},
		storageAttachmentCache: make(map[names.StorageTag]jujuc.ContextStorageAttachment),
	}
	payloadCtx, err := payloads.NewContext(f.payloads)
	if err != nil {
		return nil, err
	}
	ctx.PayloadsHookContext = payloadCtx
	if err := f.updateContext(ctx); err != nil {
		return nil, err
	}
	return ctx, nil
}

// ActionContext is part of the ContextFactory interface.
func (f *contextFactory) ActionContext(actionData *ActionData) (*HookContext, error) {
	if actionData == nil {
		return nil, errors.New("nil actionData specified")
	}
	ctx, err := f.coreContext()
	if err != nil {
		return nil, errors.Trace(err)
	}
	ctx.actionData = actionData
	ctx.id, err = f.newId(actionData.Name)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return ctx, nil
}

// HookContext is part of the ContextFactory interface.
func (f *contextFactory) HookContext(hookInfo hook.Info) (*HookContext, error) {
	ctx, err := f.coreContext()
	if err != nil {
		return nil, errors.Trace(err)
	}
	hookName := string(hookInfo.Kind)
	if hookInfo.Kind.IsRelation() {
		ctx.relationId = hookInfo.RelationId
		ctx.remoteUnitName = hookInfo.RemoteUnit
		ctx.remoteApplicationName = hookInfo.RemoteApplication
		ctx.departingUnitName = hookInfo.DepartingUnit
		relation, found := ctx.relations[hookInfo.RelationId]
		if !found {
			return nil, errors.Errorf("unknown relation id: %v", hookInfo.RelationId)
		}
		if hookInfo.Kind == hooks.RelationDeparted {
			relation.cache.RemoveMember(hookInfo.RemoteUnit)
		} else if hookInfo.RemoteUnit != "" {
			// Clear remote settings cache for changing remote unit.
			relation.cache.InvalidateMember(hookInfo.RemoteUnit)
		} else if hookInfo.RemoteApplication != "" {
			// relation.cache.InvalidateApplication(hookInfo.RemoteApplication)
		}
		hookName = fmt.Sprintf("%s-%s", relation.Name(), hookInfo.Kind)
	}
	if hookInfo.Kind.IsStorage() {
		ctx.storageTag = names.NewStorageTag(hookInfo.StorageId)
		storageName, err := names.StorageName(hookInfo.StorageId)
		if err != nil {
			return nil, errors.Trace(err)
		}
		hookName = fmt.Sprintf("%s-%s", storageName, hookName)
		// Cache the storage this hook context is for.
		_, err = ctx.Storage(ctx.storageTag)
		if err != nil && !errors.Is(err, errors.NotProvisioned) {
			return nil, errors.Annotatef(err, "could not retrieve storage for id: %v", hookInfo.StorageId)
		}
	}
	if hookInfo.Kind.IsWorkload() {
		ctx.workloadName = hookInfo.WorkloadName
		hookName = fmt.Sprintf("%s-%s", hookInfo.WorkloadName, hookName)
		switch hookInfo.Kind {
		case hooks.PebbleCustomNotice:
			ctx.noticeID = hookInfo.NoticeID
			ctx.noticeType = hookInfo.NoticeType
			ctx.noticeKey = hookInfo.NoticeKey
		case hooks.PebbleCheckFailed, hooks.PebbleCheckRecovered:
			ctx.checkName = hookInfo.CheckName
		}
	}
	if hookInfo.Kind == hooks.PreSeriesUpgrade {
		ctx.baseUpgradeTarget = hookInfo.MachineUpgradeTarget
	}
	if hookInfo.Kind.IsSecret() {
		ctx.secretURI = hookInfo.SecretURI
		ctx.secretLabel = hookInfo.SecretLabel
		if hook.SecretHookRequiresRevision(hookInfo.Kind) {
			ctx.secretRevision = hookInfo.SecretRevision
		}
		if ctx.secretLabel == "" {
			info, err := ctx.SecretMetadata()
			if err != nil {
				return nil, errors.Trace(err)
			}
			uri, err := secrets.ParseURI(ctx.secretURI)
			if err != nil {
				return nil, errors.Trace(err)
			}
			md := info[uri.ID]
			ctx.secretLabel = md.Label
		}
	}
	ctx.id, err = f.newId(hookName)
	if err != nil {
		return nil, errors.Trace(err)
	}
	ctx.hookName = hookName
	return ctx, nil
}

// CommandContext is part of the ContextFactory interface.
func (f *contextFactory) CommandContext(commandInfo CommandInfo) (*HookContext, error) {
	ctx, err := f.coreContext()
	if err != nil {
		return nil, errors.Trace(err)
	}
	// TODO(jam): 2019-10-24 Include remoteAppName
	relationId, remoteUnitName, err := inferRemoteUnit(ctx.relations, commandInfo)
	if err != nil {
		return nil, errors.Trace(err)
	}
	ctx.relationId = relationId
	ctx.remoteUnitName = remoteUnitName
	ctx.id, err = f.newId("run-commands")
	if err != nil {
		return nil, errors.Trace(err)
	}
	return ctx, nil
}

// ModelType is part of the ContextFactory interface.
func (f *contextFactory) ModelType() model.ModelType {
	return f.modelType
}

// getContextRelations updates the factory's relation caches, and uses them
// to construct ContextRelations for a fresh context.
func (f *contextFactory) getContextRelations() map[int]*ContextRelation {
	contextRelations := map[int]*ContextRelation{}
	relationInfos := f.getRelationInfos()
	relationCaches := map[int]*RelationCache{}
	for id, info := range relationInfos {
		relationUnit := info.RelationUnit
		memberNames := info.MemberNames
		cache, found := f.relationCaches[id]
		if found {
			cache.Prune(memberNames)
		} else {
			cache = NewRelationCache(relationUnit.ReadSettings, memberNames)
		}
		relationCaches[id] = cache
		// If there are no members and the relation is dying or suspended, a relation is broken.
		rel := info.RelationUnit.Relation()
		relationInactive := rel.Life() != life.Alive || rel.Suspended()
		isPeer := info.RelationUnit.Endpoint().Role == charm.RolePeer
		broken := !isPeer && relationInactive && len(memberNames) == 0
		contextRelations[id] = NewContextRelation(relationUnit, cache, broken)
	}
	f.relationCaches = relationCaches
	return contextRelations
}

// updateContext fills in all unspecialized fields that require an API call to
// discover.
//
// Approximately *every* line of code in this function represents a bug: ie, some
// piece of information we expose to the charm but which we fail to report changes
// to via hooks. Furthermore, the fact that we make multiple API calls at this
// time, rather than grabbing everything we need in one go, is unforgivably yucky.
func (f *contextFactory) updateContext(ctx *HookContext) (err error) {
	defer func() { err = errors.Trace(err) }()

	ctx.apiAddrs, err = f.state.APIAddresses()
	if err != nil {
		return err
	}

	// SLA level is removed in 4.0, so the facade is not available.
	// Setting the SLA level to empty string is a valid SLA level (slaNone).
	sla, err := f.state.SLALevel()
	if err != nil && !errors.Is(err, errors.NotImplemented) {
		return errors.Annotate(err, "could not retrieve the SLA level")
	}
	ctx.slaLevel = sla

	apiVersion, err := f.state.CloudAPIVersion()
	if err != nil {
		f.logger.Warningf("could not retrieve the cloud API version: %v", err)
	}
	ctx.cloudAPIVersion = apiVersion

	// TODO(fwereade) 23-10-2014 bug 1384572
	// Nothing here should ever be getting the environ config directly.
	modelConfig, err := f.state.ModelConfig()
	if err != nil {
		return err
	}
	ctx.legacyProxySettings = modelConfig.LegacyProxySettings()
	ctx.jujuProxySettings = modelConfig.JujuProxySettings()

	// MeterStatus is removed in 4.0, so the facade is not available.
	// Setting meter status code and info to be empty string should be
	// valid.
	statusCode, statusInfo, err := f.unit.MeterStatus()
	if err != nil && !errors.Is(err, errors.NotImplemented) {
		return errors.Annotate(err, "could not retrieve meter status for unit")
	}
	ctx.meterStatus = &meterStatus{
		code: statusCode,
		info: statusInfo,
	}

	var machPortRanges map[names.UnitTag]network.GroupedPortRanges
	var appPortRanges map[names.UnitTag]network.GroupedPortRanges
	switch f.modelType {
	case model.IAAS:
		if machPortRanges, err = f.state.OpenedMachinePortRangesByEndpoint(f.machineTag); err != nil {
			return errors.Trace(err)
		}

		ctx.privateAddress, err = f.unit.PrivateAddress()
		if err != nil && !params.IsCodeNoAddressSet(err) {
			f.logger.Warningf("cannot get legacy private address for %v: %v", f.unit.Name(), err)
		}
	case model.CAAS:
		if appPortRanges, err = f.state.OpenedPortRangesByEndpoint(); err != nil && !errors.Is(err, errors.NotSupported) {
			return errors.Trace(err)
		}
	}

	ctx.portRangeChanges = newPortRangeChangeRecorder(ctx.logger, f.unit.Tag(), f.modelType, machPortRanges, appPortRanges)
	ctx.secretChanges = newSecretsChangeRecorder(ctx.logger)
	info, err := ctx.secretsClient.SecretMetadata()
	if err != nil {
		return err
	}
	ctx.secretMetadata = make(map[string]jujuc.SecretMetadata)
	for _, v := range info {
		md := v.Metadata
		ownerTag, err := names.ParseTag(md.OwnerTag)
		if err != nil {
			return err
		}
		ctx.secretMetadata[md.URI.ID] = jujuc.SecretMetadata{
			Description:      md.Description,
			Label:            md.Label,
			Owner:            ownerTag,
			RotatePolicy:     md.RotatePolicy,
			LatestRevision:   md.LatestRevision,
			LatestChecksum:   md.LatestRevisionChecksum,
			LatestExpireTime: md.LatestExpireTime,
			NextRotateTime:   md.NextRotateTime,
			Revisions:        v.Revisions,
			Access:           md.Access,
		}
	}

	return nil
}

func inferRemoteUnit(rctxs map[int]*ContextRelation, info CommandInfo) (int, string, error) {
	relationId := info.RelationId
	hasRelation := relationId != -1
	remoteUnit := info.RemoteUnitName
	hasRemoteUnit := remoteUnit != ""

	// Check baseline sanity of remote unit, if supplied.
	if hasRemoteUnit {
		if !names.IsValidUnit(remoteUnit) {
			return -1, "", errors.Errorf(`invalid remote unit: %s`, remoteUnit)
		} else if !hasRelation {
			return -1, "", errors.Errorf("remote unit provided without a relation: %s", remoteUnit)
		}
	}

	// Check sanity of relation, if supplied, otherwise easy early return.
	if !hasRelation {
		return relationId, remoteUnit, nil
	}
	rctx, found := rctxs[relationId]
	if !found {
		return -1, "", errors.Errorf("unknown relation id: %d", relationId)
	}

	// Past basic sanity checks; if forced, accept what we're given.
	if info.ForceRemoteUnit {
		return relationId, remoteUnit, nil
	}

	// Infer an appropriate remote unit if we can.
	possibles := rctx.UnitNames()
	if remoteUnit == "" {
		switch len(possibles) {
		case 0:
			return -1, "", errors.Errorf("cannot infer remote unit in empty relation %d", relationId)
		case 1:
			return relationId, possibles[0], nil
		}
		return -1, "", errors.Errorf("ambiguous remote unit; possibilities are %+v", possibles)
	}
	for _, possible := range possibles {
		if remoteUnit == possible {
			return relationId, remoteUnit, nil
		}
	}
	return -1, "", errors.Errorf("unknown remote unit %s; possibilities are %+v", remoteUnit, possibles)
}
