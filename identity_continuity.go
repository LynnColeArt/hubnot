package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

const (
	IdentityEdgePending     = "pending"
	IdentityEdgeAccepted    = "accepted"
	IdentityEdgeConflicting = "conflicting"

	IdentityActorActive    = "active"
	IdentityActorRetired   = "retired"
	IdentityActorAmbiguous = "ambiguous"

	identityRotationCommandVersion = 1
)

type identityRotationCommandState struct {
	Version int                   `json:"version"`
	State   identityRotationState `json:"state"`
	Target  Identity              `json:"target"`
}

type IdentityContinuityEdge struct {
	AuthorizationID  string   `json:"authorizationId"`
	AuthorizingActor string   `json:"authorizingActor"`
	TargetActor      string   `json:"targetActor"`
	TargetKey        string   `json:"targetKey"`
	Relationship     string   `json:"relationship"`
	State            string   `json:"state"`
	AcceptanceIDs    []string `json:"acceptanceIds"`
}

type IdentityContinuityConflict struct {
	Kind     string   `json:"kind"`
	ActorIDs []string `json:"actorIds"`
	EventIDs []string `json:"eventIds"`
}

type IdentityContinuityMissing struct {
	AcceptanceID string `json:"acceptanceId"`
	SubjectID    string `json:"subjectId"`
}

type IdentityContinuityActor struct {
	Actor                string   `json:"actor"`
	State                string   `json:"state"`
	AcceptedDeviceIDs    []string `json:"acceptedDeviceIds"`
	AcceptedSuccessorIDs []string `json:"acceptedSuccessorIds"`
}

type IdentityContinuityProjection struct {
	Edges     []IdentityContinuityEdge     `json:"edges"`
	Actors    []IdentityContinuityActor    `json:"actors"`
	Conflicts []IdentityContinuityConflict `json:"conflicts"`
	Missing   []IdentityContinuityMissing  `json:"missing"`
}

func (projection IdentityContinuityProjection) Actor(actor string) *IdentityContinuityActor {
	index := sort.Search(len(projection.Actors), func(index int) bool {
		return projection.Actors[index].Actor >= actor
	})
	if index == len(projection.Actors) || projection.Actors[index].Actor != actor {
		return nil
	}
	return &projection.Actors[index]
}

func ProjectIdentityContinuity(events []StoredEvent) (IdentityContinuityProjection, error) {
	projection := IdentityContinuityProjection{
		Edges:     []IdentityContinuityEdge{},
		Actors:    []IdentityContinuityActor{},
		Conflicts: []IdentityContinuityConflict{},
		Missing:   []IdentityContinuityMissing{},
	}
	authorizations := make(map[string]StoredEvent)
	acceptances := make(map[string][]StoredEvent)
	actors := make(map[string]struct{})

	for _, stored := range events {
		if stored.Event.Kind != "identity.authorize" && stored.Event.Kind != "identity.accept" {
			continue
		}
		verified, id, err := verifyEvent(stored.Payload, stored.Signature)
		if err != nil {
			return IdentityContinuityProjection{}, fmt.Errorf("verify identity continuity event %s: %w", shortID(stored.ID), err)
		}
		if id != stored.ID || !reflect.DeepEqual(verified, stored.Event) {
			return IdentityContinuityProjection{}, fmt.Errorf("identity continuity event %s does not match its verified payload", shortID(stored.ID))
		}
		actors[verified.Actor] = struct{}{}
		if verified.Kind == "identity.authorize" {
			authorizations[id] = stored
			actors[verified.TargetActor] = struct{}{}
		} else {
			acceptances[verified.Subject] = append(acceptances[verified.Subject], stored)
		}
	}

	authorizationIDs := make([]string, 0, len(authorizations))
	for id := range authorizations {
		authorizationIDs = append(authorizationIDs, id)
	}
	sort.Strings(authorizationIDs)
	edgeIndex := make(map[string]int, len(authorizationIDs))
	acceptedSuccessors := make(map[string][]string)
	acceptedTargets := make(map[string]string)

	for _, id := range authorizationIDs {
		authorization := authorizations[id]
		edge := IdentityContinuityEdge{
			AuthorizationID:  id,
			AuthorizingActor: authorization.Event.Actor,
			TargetActor:      authorization.Event.TargetActor,
			TargetKey:        authorization.Event.TargetKey,
			Relationship:     authorization.Event.Relationship,
			State:            IdentityEdgePending,
			AcceptanceIDs:    []string{},
		}
		for _, acceptance := range acceptances[id] {
			if acceptance.Event.Actor != authorization.Event.TargetActor || acceptance.Event.PublicKey != authorization.Event.TargetKey {
				projection.Conflicts = append(projection.Conflicts, IdentityContinuityConflict{
					Kind: "acceptance-signer-mismatch", ActorIDs: sortedUniqueStrings(authorization.Event.Actor, acceptance.Event.Actor),
					EventIDs: []string{id, acceptance.ID},
				})
				continue
			}
			edge.AcceptanceIDs = append(edge.AcceptanceIDs, acceptance.ID)
		}
		sort.Strings(edge.AcceptanceIDs)
		if len(edge.AcceptanceIDs) > 0 {
			edge.State = IdentityEdgeAccepted
			if edge.Relationship == identityRelationshipSuccessor {
				acceptedSuccessors[edge.AuthorizingActor] = append(acceptedSuccessors[edge.AuthorizingActor], id)
				acceptedTargets[id] = edge.TargetActor
			}
		}
		edgeIndex[id] = len(projection.Edges)
		projection.Edges = append(projection.Edges, edge)
	}

	for subject, values := range acceptances {
		if _, exists := authorizations[subject]; exists {
			continue
		}
		for _, acceptance := range values {
			projection.Missing = append(projection.Missing, IdentityContinuityMissing{
				AcceptanceID: acceptance.ID,
				SubjectID:    subject,
			})
		}
	}
	sort.Slice(projection.Missing, func(i, j int) bool {
		if projection.Missing[i].SubjectID != projection.Missing[j].SubjectID {
			return projection.Missing[i].SubjectID < projection.Missing[j].SubjectID
		}
		return projection.Missing[i].AcceptanceID < projection.Missing[j].AcceptanceID
	})

	ambiguousActors := make(map[string]bool)
	for actor, ids := range acceptedSuccessors {
		sort.Strings(ids)
		if len(ids) <= 1 {
			continue
		}
		ambiguousActors[actor] = true
		projection.Conflicts = append(projection.Conflicts, IdentityContinuityConflict{
			Kind: "competing-successors", ActorIDs: []string{actor}, EventIDs: append([]string(nil), ids...),
		})
		for _, id := range ids {
			projection.Edges[edgeIndex[id]].State = IdentityEdgeConflicting
		}
	}

	cycleIDs := successorCycleIDs(acceptedSuccessors, acceptedTargets)
	for _, ids := range cycleIDs {
		cycleActors := make([]string, 0, len(ids))
		for _, id := range ids {
			edge := &projection.Edges[edgeIndex[id]]
			edge.State = IdentityEdgeConflicting
			ambiguousActors[edge.AuthorizingActor] = true
			cycleActors = append(cycleActors, edge.AuthorizingActor)
		}
		sort.Strings(ids)
		sort.Strings(cycleActors)
		projection.Conflicts = append(projection.Conflicts, IdentityContinuityConflict{
			Kind: "successor-cycle", ActorIDs: cycleActors, EventIDs: ids,
		})
	}

	actorIDs := make([]string, 0, len(actors))
	for actor := range actors {
		actorIDs = append(actorIDs, actor)
	}
	sort.Strings(actorIDs)
	for _, actor := range actorIDs {
		state := IdentityContinuityActor{Actor: actor, State: IdentityActorActive, AcceptedDeviceIDs: []string{}, AcceptedSuccessorIDs: []string{}}
		for _, edge := range projection.Edges {
			if edge.State != IdentityEdgeAccepted {
				continue
			}
			if edge.Relationship == identityRelationshipDevice && (edge.AuthorizingActor == actor || edge.TargetActor == actor) {
				state.AcceptedDeviceIDs = append(state.AcceptedDeviceIDs, edge.AuthorizationID)
			}
			if edge.Relationship == identityRelationshipSuccessor && edge.AuthorizingActor == actor {
				state.AcceptedSuccessorIDs = append(state.AcceptedSuccessorIDs, edge.AuthorizationID)
			}
		}
		if ambiguousActors[actor] {
			state.State = IdentityActorAmbiguous
		} else if len(state.AcceptedSuccessorIDs) == 1 {
			state.State = IdentityActorRetired
		}
		projection.Actors = append(projection.Actors, state)
	}

	for index := range projection.Conflicts {
		sort.Strings(projection.Conflicts[index].ActorIDs)
		sort.Strings(projection.Conflicts[index].EventIDs)
	}
	sort.Slice(projection.Conflicts, func(i, j int) bool {
		left, right := projection.Conflicts[i], projection.Conflicts[j]
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		leftActors, rightActors := strings.Join(left.ActorIDs, "\x00"), strings.Join(right.ActorIDs, "\x00")
		if leftActors != rightActors {
			return leftActors < rightActors
		}
		return strings.Join(left.EventIDs, "\x00") < strings.Join(right.EventIDs, "\x00")
	})
	return projection, nil
}

func sortedUniqueStrings(values ...string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func successorCycleIDs(outgoing map[string][]string, targets map[string]string) [][]string {
	actorsSet := make(map[string]struct{})
	for actor, ids := range outgoing {
		actorsSet[actor] = struct{}{}
		sort.Strings(ids)
		outgoing[actor] = ids
		for _, id := range ids {
			actorsSet[targets[id]] = struct{}{}
		}
	}
	actors := make([]string, 0, len(actorsSet))
	for actor := range actorsSet {
		actors = append(actors, actor)
	}
	sort.Strings(actors)

	index := 0
	indices := make(map[string]int)
	lowlinks := make(map[string]int)
	onStack := make(map[string]bool)
	stack := []string{}
	components := [][]string{}
	var connect func(string)
	connect = func(actor string) {
		indices[actor] = index
		lowlinks[actor] = index
		index++
		stack = append(stack, actor)
		onStack[actor] = true
		for _, id := range outgoing[actor] {
			target := targets[id]
			targetIndex, seen := indices[target]
			if !seen {
				connect(target)
				if lowlinks[target] < lowlinks[actor] {
					lowlinks[actor] = lowlinks[target]
				}
			} else if onStack[target] && targetIndex < lowlinks[actor] {
				lowlinks[actor] = targetIndex
			}
		}
		if lowlinks[actor] != indices[actor] {
			return
		}
		component := []string{}
		for {
			last := len(stack) - 1
			member := stack[last]
			stack = stack[:last]
			onStack[member] = false
			component = append(component, member)
			if member == actor {
				break
			}
		}
		if len(component) > 1 {
			components = append(components, component)
		}
	}
	for _, actor := range actors {
		if _, seen := indices[actor]; !seen {
			connect(actor)
		}
	}

	cycles := make([][]string, 0, len(components))
	for _, component := range components {
		members := make(map[string]bool, len(component))
		for _, actor := range component {
			members[actor] = true
		}
		ids := []string{}
		for _, actor := range component {
			for _, id := range outgoing[actor] {
				if members[targets[id]] {
					ids = append(ids, id)
				}
			}
		}
		sort.Strings(ids)
		cycles = append(cycles, ids)
	}
	sort.Slice(cycles, func(i, j int) bool {
		return strings.Join(cycles[i], "\x00") < strings.Join(cycles[j], "\x00")
	})
	return cycles
}

// inspectIdentity validates the existing signer without initializing or migrating
// private keyring state. Human commands retain the ordinary migration loader.
func inspectIdentity() (*Identity, error) {
	actor, err := loadActiveActor()
	if err == nil {
		return loadIdentityRecord(actor)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return loadLegacyIdentity()
}

func cmdIdentityShow(args []string) error {
	flags := flag.NewFlagSet("identity show", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	machine := flags.Bool("json", false, "JSON output")
	requested := issueMachineRequested(args, flags)
	err := flags.Parse(args)
	if err == nil && flags.NArg() != 0 {
		err = errors.New("unexpected identity show arguments")
	}
	fail := func(code string, cause error) error {
		failure := issueFailure(code, cause)
		if requested {
			var typed *issueCommandError
			errors.As(failure, &typed)
			if err := json.NewEncoder(os.Stdout).Encode(issueEnvelope{Schema: "hn.identity/1", OK: false, Error: typed}); err != nil {
				return err
			}
		}
		return failure
	}
	if err != nil {
		return fail("invalid_input", err)
	}
	var identity *Identity
	if *machine {
		identity, err = inspectIdentity()
	} else {
		identity, err = loadIdentity()
	}
	if err != nil {
		if !requested {
			return err
		}
		return fail("repository_error", errors.New("active identity unavailable; inspect identity configuration"))
	}
	if *machine {
		public := struct {
			Actor     string `json:"actor"`
			Name      string `json:"name"`
			PublicKey string `json:"public_key"`
		}{identity.Actor, identity.Name, identity.PublicKey}
		return json.NewEncoder(os.Stdout).Encode(issueEnvelope{Schema: "hn.identity/1", OK: true, Data: public})
	}
	fmt.Printf("Name:   %s\n", oneLine(identity.Name))
	fmt.Printf("Actor:  %s\n", identity.Actor)
	fmt.Printf("Ref:    %s\n", actorRef(identity.Actor))
	return nil
}

func cmdIdentityPublic(args []string) error {
	if len(args) != 0 {
		return usageError("usage: hn identity public")
	}
	identity, err := loadIdentity()
	if err != nil {
		return err
	}
	fmt.Printf("Name:       %s\n", oneLine(identity.Name))
	fmt.Printf("Actor:      %s\n", identity.Actor)
	fmt.Printf("Public key: %s\n", identity.PublicKey)
	return nil
}

func cmdIdentityList(args []string) error {
	if len(args) != 0 {
		return usageError("usage: hn identity list")
	}
	records, err := listIdentityRecords()
	if err != nil {
		return err
	}
	events, err := collectEvents()
	if err != nil {
		return err
	}
	projection, err := ProjectIdentityContinuity(events)
	if err != nil {
		return err
	}
	for _, record := range records {
		lifecycle := "stored"
		if record.Active {
			lifecycle = "active"
		} else if actor := projection.Actor(record.Actor); actor != nil && actor.State == IdentityActorRetired {
			lifecycle = "retired"
		}
		fmt.Printf("%s  %-7s  %s\n", record.Actor, lifecycle, oneLine(record.Name))
	}
	for _, actor := range projection.Actors {
		fmt.Printf("Public actor: %s state=%s\n", actor.Actor, actor.State)
	}
	for _, edge := range projection.Edges {
		acceptances := "(none)"
		if len(edge.AcceptanceIDs) > 0 {
			acceptances = strings.Join(edge.AcceptanceIDs, ",")
		}
		fmt.Printf("Relationship: %s %s %s -> %s state=%s acceptances=%s\n",
			edge.AuthorizationID, edge.Relationship, edge.AuthorizingActor, edge.TargetActor, edge.State, acceptances)
	}
	for _, conflict := range projection.Conflicts {
		fmt.Printf("Identity conflict: %s actors=%s events=%s authority=not-inferred\n",
			conflict.Kind, strings.Join(conflict.ActorIDs, ","), strings.Join(conflict.EventIDs, ","))
	}
	for _, missing := range projection.Missing {
		fmt.Printf("Missing identity authorization: acceptance=%s subject=%s authority=not-inferred\n",
			missing.AcceptanceID, missing.SubjectID)
	}
	return nil
}

func cmdIdentityAuthorize(args []string) error {
	flags := quietFlags("identity authorize")
	relationship := flags.String("relationship", "", "device or successor")
	actor := flags.String("actor", "", "full target actor")
	publicKey := flags.String("public-key", "", "raw-base64 target public key")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return usageError("usage: hn identity authorize --relationship device|successor --actor ACTOR --public-key KEY")
	}
	identity, err := loadIdentity()
	if err != nil {
		return err
	}
	event, err := nextEvent(identity, "identity.authorize")
	if err != nil {
		return err
	}
	event.Relationship = *relationship
	event.TargetActor = *actor
	event.TargetKey = *publicKey
	stored, err := appendEvent(event, identity)
	if err != nil {
		return err
	}
	fmt.Printf("Authorized %s identity %s (%s)\n", event.Relationship, event.TargetActor, stored.ID)
	return nil
}

func cmdIdentityAccept(args []string) error {
	if len(args) != 1 || !validEventID(args[0]) {
		return usageError("usage: hn identity accept FULL_AUTHORIZATION_EVENT_ID")
	}
	identity, err := loadIdentity()
	if err != nil {
		return err
	}
	events, err := collectEvents()
	if err != nil {
		return err
	}
	authorization, err := resolveFullEvent(events, args[0])
	if err != nil {
		return err
	}
	if authorization.Event.Kind != "identity.authorize" {
		return fmt.Errorf("event %s is not an identity authorization", shortID(authorization.ID))
	}
	if identity.Actor != authorization.Event.TargetActor || identity.PublicKey != authorization.Event.TargetKey {
		return fmt.Errorf("active identity is not the exact target of authorization %s", shortID(authorization.ID))
	}
	event, err := nextEvent(identity, "identity.accept")
	if err != nil {
		return err
	}
	event.Subject = authorization.ID
	stored, err := appendEvent(event, identity)
	if err != nil {
		return err
	}
	fmt.Printf("Accepted identity authorization %s (%s)\n", authorization.ID, stored.ID)
	return nil
}

func resolveFullEvent(events []StoredEvent, id string) (*StoredEvent, error) {
	if !validEventID(id) {
		return nil, fmt.Errorf("event ID must be full and valid")
	}
	for index := range events {
		if events[index].ID == id {
			return &events[index], nil
		}
	}
	return nil, fmt.Errorf("event %q not found", id)
}

func cmdIdentityRotate(args []string) error {
	flags := quietFlags("identity rotate")
	name := flags.String("name", "", "new identity display name")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return usageError("usage: hn identity rotate [--name NAME]")
	}
	state, predecessor, target, completed, err := prepareIdentityRotation(*name)
	if err != nil {
		return err
	}
	if completed {
		if err := discardCompletedIdentityRotationCommand(); err != nil {
			return err
		}
		fmt.Printf("Rotated identity from %s to %s\n", state.PredecessorActor, state.TargetActor)
		return nil
	}
	if err := validateIdentityRotationProgress(state, target, false); err != nil {
		return err
	}
	if state.AuthorizationEvent == "" {
		stored, err := findOrAppendRotationAuthorization(state, predecessor, target)
		if err != nil {
			return err
		}
		if err := runIdentityStorageHook("rotation-authorization-durable"); err != nil {
			return err
		}
		state.AuthorizationEvent = stored.ID
		if err := storeIdentityRotation(state); err != nil {
			return err
		}
		if err := updateIdentityRotationCommand(state, target); err != nil {
			return err
		}
	}
	if state.AcceptanceEvent == "" {
		stored, err := findOrAppendRotationAcceptance(state, target)
		if err != nil {
			return err
		}
		if err := runIdentityStorageHook("rotation-acceptance-durable"); err != nil {
			return err
		}
		state.AcceptanceEvent = stored.ID
		if err := storeIdentityRotation(state); err != nil {
			return err
		}
		if err := updateIdentityRotationCommand(state, target); err != nil {
			return err
		}
	}
	if err := switchActiveIdentityWithVerifiedEvents(state, target); err != nil {
		return err
	}
	if err := discardCompletedIdentityRotationCommand(); err != nil {
		return err
	}
	fmt.Printf("Rotated identity from %s to %s\n", state.PredecessorActor, state.TargetActor)
	return nil
}

func prepareIdentityRotation(name string) (identityRotationState, *Identity, *Identity, bool, error) {
	command, commandErr := loadIdentityRotationCommand()
	state, rotationErr := loadIdentityRotation()
	if commandErr == nil {
		predecessor, err := loadIdentityRecord(command.State.PredecessorActor)
		if err != nil {
			return command.State, nil, nil, false, err
		}
		target := command.Target
		storedTarget, err := loadIdentityRecord(target.Actor)
		if errors.Is(err, os.ErrNotExist) {
			if _, err := storeIdentityRecord(&target, identityLifecycleAvailable); err != nil {
				return command.State, nil, nil, false, err
			}
		} else if err != nil {
			return command.State, nil, nil, false, err
		} else if *storedTarget != target {
			return command.State, nil, nil, false, fmt.Errorf("rotation target record differs from its durable command journal")
		}

		if rotationErr == nil {
			merged, err := reconcileIdentityRotationStates(command.State, state)
			if err != nil {
				return command.State, nil, nil, false, err
			}
			if err := updateIdentityRotationCommand(merged, &target); err != nil {
				return merged, nil, nil, false, err
			}
			return merged, predecessor, &target, false, nil
		}
		if !errors.Is(rotationErr, os.ErrNotExist) {
			return command.State, nil, nil, false, rotationErr
		}
		active, err := loadActiveActor()
		if err != nil {
			return command.State, nil, nil, false, err
		}
		if active == command.State.TargetActor {
			if err := validateIdentityRotationProgress(command.State, &target, true); err != nil {
				return command.State, nil, nil, false, err
			}
			return command.State, predecessor, &target, true, nil
		}
		if active != command.State.PredecessorActor {
			return command.State, nil, nil, false, fmt.Errorf("active identity is %s, not rotation predecessor %s", active, command.State.PredecessorActor)
		}
		if err := storeIdentityRotation(command.State); err != nil {
			return command.State, nil, nil, false, err
		}
		return command.State, predecessor, &target, false, nil
	}
	if !errors.Is(commandErr, os.ErrNotExist) {
		return identityRotationState{}, nil, nil, false, commandErr
	}
	if rotationErr == nil {
		predecessor, err := loadIdentityRecord(state.PredecessorActor)
		if err != nil {
			return state, nil, nil, false, err
		}
		target, err := loadIdentityRecord(state.TargetActor)
		if err != nil {
			return state, nil, nil, false, err
		}
		if err := updateIdentityRotationCommand(state, target); err != nil {
			return state, nil, nil, false, err
		}
		return state, predecessor, target, false, nil
	}
	if !errors.Is(rotationErr, os.ErrNotExist) {
		return identityRotationState{}, nil, nil, false, rotationErr
	}
	predecessor, err := loadActiveIdentity()
	if err != nil {
		return identityRotationState{}, nil, nil, false, err
	}
	targetName := strings.TrimSpace(name)
	if targetName == "" {
		targetName = predecessor.Name
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return identityRotationState{}, nil, nil, false, fmt.Errorf("generate rotation identity: %w", err)
	}
	target := &Identity{
		Actor: actorForPublicKey(publicKey), Name: targetName,
		PublicKey:  base64.RawStdEncoding.EncodeToString(publicKey),
		PrivateKey: base64.RawStdEncoding.EncodeToString(privateKey),
	}
	state = identityRotationState{
		Version: identityRotationVersion, PredecessorActor: predecessor.Actor,
		TargetActor: target.Actor, Relationship: identityRelationshipSuccessor,
	}
	if err := updateIdentityRotationCommand(state, target); err != nil {
		return identityRotationState{}, nil, nil, false, err
	}
	if err := runIdentityStorageHook("rotation-target-intent-durable"); err != nil {
		return identityRotationState{}, nil, nil, false, err
	}
	if _, err := storeIdentityRecord(target, identityLifecycleAvailable); err != nil {
		return identityRotationState{}, nil, nil, false, err
	}
	if err := runIdentityStorageHook("rotation-target-record-durable"); err != nil {
		return identityRotationState{}, nil, nil, false, err
	}
	if err := storeIdentityRotation(state); err != nil {
		return identityRotationState{}, nil, nil, false, err
	}
	return state, predecessor, target, false, nil
}

func identityRotationCommandPath() (string, error) {
	paths, err := identityKeyringPaths()
	if err != nil {
		return "", err
	}
	return filepath.Join(paths.root, "rotation-command.json"), nil
}

func validateIdentityRotationCommand(command identityRotationCommandState) error {
	if command.Version != identityRotationCommandVersion {
		return fmt.Errorf("unsupported identity rotation command version %d", command.Version)
	}
	if err := validateRotationState(command.State, false); err != nil {
		return err
	}
	if err := validateIdentity(&command.Target); err != nil {
		return fmt.Errorf("invalid rotation command target: %w", err)
	}
	if command.Target.Actor != command.State.TargetActor {
		return fmt.Errorf("rotation command target does not match its transaction actor")
	}
	return nil
}

func updateIdentityRotationCommand(state identityRotationState, target *Identity) error {
	command := identityRotationCommandState{Version: identityRotationCommandVersion, State: state, Target: *target}
	if err := validateIdentityRotationCommand(command); err != nil {
		return err
	}
	path, err := identityRotationCommandPath()
	if err != nil {
		return err
	}
	paths, err := identityKeyringPaths()
	if err != nil {
		return err
	}
	if err := ensureIdentityKeyringDirectories(paths); err != nil {
		return err
	}
	existing, err := loadIdentityRotationCommand()
	if err == nil {
		merged, err := reconcileIdentityRotationStates(existing.State, state)
		if err != nil {
			return err
		}
		if existing.Target != *target {
			return fmt.Errorf("identity rotation command target cannot be replaced")
		}
		command.State = merged
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	contents, err := json.MarshalIndent(command, "", "  ")
	if err != nil {
		return err
	}
	return writePrivateFileAtomic(path, append(contents, '\n'))
}

func loadIdentityRotationCommand() (identityRotationCommandState, error) {
	path, err := identityRotationCommandPath()
	if err != nil {
		return identityRotationCommandState{}, err
	}
	paths, err := identityKeyringPaths()
	if err != nil {
		return identityRotationCommandState{}, err
	}
	if err := checkPrivateDirectory(paths.root); err != nil {
		return identityRotationCommandState{}, err
	}
	contents, err := readPrivateFile(path)
	if err != nil {
		return identityRotationCommandState{}, err
	}
	var command identityRotationCommandState
	if err := decodePrivateJSON(contents, &command, "identity rotation command"); err != nil {
		return identityRotationCommandState{}, err
	}
	if err := validateIdentityRotationCommand(command); err != nil {
		return identityRotationCommandState{}, err
	}
	return command, nil
}

func reconcileIdentityRotationStates(left, right identityRotationState) (identityRotationState, error) {
	if left.Version != right.Version || left.PredecessorActor != right.PredecessorActor ||
		left.TargetActor != right.TargetActor || left.Relationship != right.Relationship {
		return identityRotationState{}, fmt.Errorf("identity rotation command does not match durable rotation state")
	}
	merged := left
	for description, values := range map[string][2]string{
		"authorization": {left.AuthorizationEvent, right.AuthorizationEvent},
		"acceptance":    {left.AcceptanceEvent, right.AcceptanceEvent},
	} {
		if values[0] != "" && values[1] != "" && values[0] != values[1] {
			return identityRotationState{}, fmt.Errorf("identity rotation %s event cannot be replaced", description)
		}
	}
	if merged.AuthorizationEvent == "" {
		merged.AuthorizationEvent = right.AuthorizationEvent
	}
	if merged.AcceptanceEvent == "" {
		merged.AcceptanceEvent = right.AcceptanceEvent
	}
	return merged, nil
}

func validateIdentityRotationProgress(state identityRotationState, target *Identity, requireComplete bool) error {
	if err := validateRotationState(state, requireComplete); err != nil {
		return err
	}
	if state.Relationship != identityRelationshipSuccessor {
		return fmt.Errorf("planned identity rotation requires a successor relationship")
	}
	if target == nil || target.Actor != state.TargetActor {
		return fmt.Errorf("identity rotation target does not match its durable state")
	}
	if err := validateIdentity(target); err != nil {
		return fmt.Errorf("invalid identity rotation target: %w", err)
	}
	if state.AuthorizationEvent == "" {
		return nil
	}
	events, err := collectEvents()
	if err != nil {
		return fmt.Errorf("verify identity rotation event store: %w", err)
	}
	authorization, err := resolveFullEvent(events, state.AuthorizationEvent)
	if err != nil {
		return fmt.Errorf("read identity rotation authorization: %w", err)
	}
	if authorization.Event.Kind != "identity.authorize" || authorization.Event.Actor != state.PredecessorActor ||
		authorization.Event.Relationship != identityRelationshipSuccessor || authorization.Event.TargetActor != target.Actor ||
		authorization.Event.TargetKey != target.PublicKey {
		return fmt.Errorf("identity rotation authorization does not match its verified predecessor, relationship, and target")
	}
	if state.AcceptanceEvent == "" {
		return nil
	}
	acceptance, err := resolveFullEvent(events, state.AcceptanceEvent)
	if err != nil {
		return fmt.Errorf("read identity rotation acceptance: %w", err)
	}
	if acceptance.Event.Kind != "identity.accept" || acceptance.Event.Actor != target.Actor ||
		acceptance.Event.PublicKey != target.PublicKey || acceptance.Event.Subject != authorization.ID {
		return fmt.Errorf("identity rotation acceptance does not match its verified target and authorization")
	}
	return nil
}

func switchActiveIdentityWithVerifiedEvents(state identityRotationState, target *Identity) error {
	if err := validateIdentityRotationProgress(state, target, true); err != nil {
		return err
	}
	return switchActiveIdentity(state)
}

func discardCompletedIdentityRotationCommand() error {
	path, err := identityRotationCommandPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove completed identity rotation command: %w", err)
	}
	paths, err := identityKeyringPaths()
	if err != nil {
		return err
	}
	// The rotation is already complete. A failed directory sync leaves either
	// no journal or a harmless receipt. Do not report a transaction failure
	// after the receipt has already been removed: that would turn a retry into
	// a second rotation, which this journal exists to prevent.
	_ = syncPrivateDirectory(paths.root)
	return nil
}

func findOrAppendRotationAuthorization(state identityRotationState, predecessor, target *Identity) (*StoredEvent, error) {
	events, err := collectEvents()
	if err != nil {
		return nil, err
	}
	var found *StoredEvent
	for index := range events {
		event := &events[index]
		if event.Event.Kind == "identity.authorize" && event.Event.Actor == predecessor.Actor &&
			event.Event.Relationship == state.Relationship && event.Event.TargetActor == target.Actor && event.Event.TargetKey == target.PublicKey {
			if found != nil && found.ID != event.ID {
				return nil, fmt.Errorf("rotation has multiple matching authorization facts")
			}
			found = event
		}
	}
	if found != nil {
		return found, nil
	}
	event, err := nextEvent(predecessor, "identity.authorize")
	if err != nil {
		return nil, err
	}
	event.Relationship = state.Relationship
	event.TargetActor = target.Actor
	event.TargetKey = target.PublicKey
	return appendEvent(event, predecessor)
}

func findOrAppendRotationAcceptance(state identityRotationState, target *Identity) (*StoredEvent, error) {
	events, err := collectEvents()
	if err != nil {
		return nil, err
	}
	var found *StoredEvent
	for index := range events {
		event := &events[index]
		if event.Event.Kind == "identity.accept" && event.Event.Actor == target.Actor && event.Event.Subject == state.AuthorizationEvent {
			if found != nil && found.ID != event.ID {
				return nil, fmt.Errorf("rotation has multiple matching acceptance facts")
			}
			found = event
		}
	}
	if found != nil {
		return found, nil
	}
	event, err := nextEvent(target, "identity.accept")
	if err != nil {
		return nil, err
	}
	event.Subject = state.AuthorizationEvent
	return appendEvent(event, target)
}
