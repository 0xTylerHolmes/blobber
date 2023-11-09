package slot_actions

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/marioevz/blobber/common"
	"github.com/marioevz/blobber/keys"
	"github.com/marioevz/blobber/p2p"
	"github.com/pkg/errors"
	beacon_common "github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/deneb"
	"github.com/protolambda/ztyp/tree"
)

const MAX_BLOBS_PER_BLOCK = 6

type SlotAction interface {
	Name() string
	Fields() map[string]interface{}
	GetTestPeerCount() int
	Execute(
		spec *beacon_common.Spec,
		testPeers p2p.TestPeers,
		beaconBlockContents *deneb.BlockContents,
		beaconBlockDomain beacon_common.BLSDomain,
		validatorKey *keys.ValidatorKey,
		includeBlobRecord *common.BlobRecord,
		rejectBlobRecord *common.BlobRecord,
	) (bool, error)
}

func UnmarshallSlotAction(data []byte) (SlotAction, error) {
	if len(data) == 0 {
		return nil, nil
	}

	type actionName struct {
		Name string `json:"name"`
	}
	var actionNameObj actionName
	if err := json.Unmarshal(data, &actionNameObj); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshall slot action name")
	}

	var action SlotAction
	switch actionNameObj.Name {
	case "default":
		action = &Default{}
	case "broadcast_blobs_before_block":
		action = &BroadcastBlobsBeforeBlock{}
	case "blob_gossip_delay":
		action = &BlobGossipDelay{}
	case "equivocating_block_and_blobs":
		action = &EquivocatingBlockAndBlobs{}
	/*
		case "extra_blobs":
			action = &ExtraBlobs{}
		case "conflicting_blobs":
			action = &ConflictingBlobs{}
		case "swap_blobs":
			action = &SwapBlobs{}
	*/
	default:
		return nil, fmt.Errorf("unknown slot action name: %s", actionNameObj.Name)
	}

	if err := json.Unmarshal(data, &action); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshall slot action")
	}
	return action, nil
}

type Default struct{}

func (s Default) Name() string {
	return "Default"
}

func (s Default) Fields() map[string]interface{} {
	return map[string]interface{}{}
}

func (s Default) GetTestPeerCount() int {
	// By default we only create 1 test p2p and it's connected to all peers
	return 1
}

func (s Default) Execute(
	spec *beacon_common.Spec,
	testPeers p2p.TestPeers,
	beaconBlockContents *deneb.BlockContents,
	beaconBlockDomain beacon_common.BLSDomain,
	validatorKey *keys.ValidatorKey,
	includeBlobRecord *common.BlobRecord,
	rejectBlobRecord *common.BlobRecord,
) (bool, error) {
	// Sign block
	signedBlockContents, err := SignBlockContents(spec, beaconBlockContents, beaconBlockDomain, validatorKey)
	if err != nil {
		return false, errors.Wrap(err, "failed to sign block")
	}
	blobSidecars, err := signedBlockContents.GenerateSidecars(spec, tree.GetHashFn())
	if err != nil {
		return false, errors.Wrap(err, "failed to generate blob sidecars")
	}

	// Broadcast the block
	if err := testPeers.BroadcastSignedBeaconBlock(spec, signedBlockContents.SignedBlock); err != nil {
		return false, errors.Wrap(err, "failed to broadcast signed beacon block")
	}

	// Broadcast the blobs
	if err := testPeers.BroadcastBlobSidecars(spec, blobSidecars...); err != nil {
		return false, errors.Wrap(err, "failed to broadcast signed blob sidecar")
	}

	// Add the blobs to the must-include blob record
	includeBlobRecord.Add(beaconBlockContents.Block.Slot, blobSidecars...)

	return true, nil
}

type BroadcastBlobsBeforeBlock struct {
	Default
}

func (s BroadcastBlobsBeforeBlock) Name() string {
	return "Broadcast blobs before block"
}

func (s BroadcastBlobsBeforeBlock) Fields() map[string]interface{} {
	return map[string]interface{}{}
}

func (s BroadcastBlobsBeforeBlock) Execute(
	spec *beacon_common.Spec,
	testPeers p2p.TestPeers,
	beaconBlockContents *deneb.BlockContents,
	beaconBlockDomain beacon_common.BLSDomain,
	validatorKey *keys.ValidatorKey,
	includeBlobRecord *common.BlobRecord,
	rejectBlobRecord *common.BlobRecord,
) (bool, error) {
	// Sign block
	signedBlockContents, err := SignBlockContents(spec, beaconBlockContents, beaconBlockDomain, validatorKey)
	if err != nil {
		return false, errors.Wrap(err, "failed to sign block")
	}
	blobSidecars, err := signedBlockContents.GenerateSidecars(spec, tree.GetHashFn())
	if err != nil {
		return false, errors.Wrap(err, "failed to generate blob sidecars")
	}

	// Broadcast the blobs
	if err := testPeers.BroadcastBlobSidecars(spec, blobSidecars...); err != nil {
		return false, errors.Wrap(err, "failed to broadcast blob sidecar")
	}

	// Broadcast the block
	if err := testPeers.BroadcastSignedBeaconBlock(spec, signedBlockContents.SignedBlock); err != nil {
		return false, errors.Wrap(err, "failed to broadcast signed beacon block")
	}

	// Add the blobs to the must-include blob record
	includeBlobRecord.Add(beaconBlockContents.Block.Slot, blobSidecars...)

	return true, nil
}

type BlobGossipDelay struct {
	Default
	DelayMilliseconds int `json:"delay_milliseconds"`
}

func (s BlobGossipDelay) Name() string {
	return "Blob gossip delay"
}

func (s BlobGossipDelay) Fields() map[string]interface{} {
	return map[string]interface{}{
		"delay_milliseconds": s.DelayMilliseconds,
	}
}

func (s BlobGossipDelay) Execute(
	spec *beacon_common.Spec,
	testPeers p2p.TestPeers,
	beaconBlockContents *deneb.BlockContents,
	beaconBlockDomain beacon_common.BLSDomain,
	validatorKey *keys.ValidatorKey,
	includeBlobRecord *common.BlobRecord,
	rejectBlobRecord *common.BlobRecord,
) (bool, error) {
	// Sign block
	signedBlockContents, err := SignBlockContents(spec, beaconBlockContents, beaconBlockDomain, validatorKey)
	if err != nil {
		return false, errors.Wrap(err, "failed to sign block")
	}
	blobSidecars, err := signedBlockContents.GenerateSidecars(spec, tree.GetHashFn())
	if err != nil {
		return false, errors.Wrap(err, "failed to generate blob sidecars")
	}

	// Broadcast the block
	if err := testPeers.BroadcastSignedBeaconBlock(spec, signedBlockContents.SignedBlock); err != nil {
		return false, errors.Wrap(err, "failed to broadcast signed beacon block")
	}

	// Insert a delay before gossiping the blobs
	time.Sleep(time.Duration(s.DelayMilliseconds) * time.Millisecond)

	// Broadcast the blobs
	if err := testPeers.BroadcastBlobSidecars(spec, blobSidecars...); err != nil {
		return false, errors.Wrap(err, "failed to broadcast signed blob sidecar")
	}

	return true, nil
}

type EquivocatingBlockAndBlobs struct {
	Default
	SendBlobsFirst      bool
	ModifyBlobs         bool
	ModifyKZGProofs     bool
	AlternateRecipients bool
}

func (s EquivocatingBlockAndBlobs) Name() string {
	return "Equivocating Block and Blobs"
}

func (s EquivocatingBlockAndBlobs) Fields() map[string]interface{} {
	return map[string]interface{}{}
}

func (s EquivocatingBlockAndBlobs) GetTestPeerCount() int {
	// We are going to send two conflicting blocks and sets of blobs through two different test p2p connections
	return 2
}

func (s EquivocatingBlockAndBlobs) Execute(
	spec *beacon_common.Spec,
	testPeers p2p.TestPeers,
	beaconBlockContents *deneb.BlockContents,
	beaconBlockDomain beacon_common.BLSDomain,
	validatorKey *keys.ValidatorKey,
	includeBlobRecord *common.BlobRecord,
	rejectBlobRecord *common.BlobRecord,
) (bool, error) {
	if len(testPeers) != 2 {
		return false, fmt.Errorf("expected 2 test p2p connections, got %d", len(testPeers))
	}

	secondBlockContents, err := CopyBlockContents(beaconBlockContents)
	if err != nil {
		return false, errors.Wrap(err, "failed to copy block contents")
	}

	// Modify the graffiti to generate a different block
	graffitiModifier := &GraffitiModifier{
		NewGraffiti: "Equiv",
		Append:      true,
	}
	graffitiModifier.ModifyBlock(spec, secondBlockContents.Block)

	beaconBlocksContents := []*deneb.BlockContents{
		beaconBlockContents,
		secondBlockContents,
	}

	// Sign the blocks and generate the sidecars
	signedBlockBlobsBundles := make([]*SignedBlockSidecarsBundle, len(beaconBlocksContents))
	for i, blockContents := range beaconBlocksContents {
		signedBlockContents, err := SignBlockContents(spec, blockContents, beaconBlockDomain, validatorKey)
		if err != nil {
			return false, errors.Wrap(err, "failed to sign block")
		}
		blobSidecars, err := signedBlockContents.GenerateSidecars(spec, tree.GetHashFn())
		if err != nil {
			return false, errors.Wrap(err, "failed to generate blob sidecars")
		}
		signedBlockBlobsBundles[i] = &SignedBlockSidecarsBundle{
			SignedBlockContents: signedBlockContents.SignedBlock,
			BlobSidecars:        blobSidecars,
		}
	}

	if s.AlternateRecipients && (beaconBlockContents.Block.Slot%2 == 0) {
		signedBlockBlobsBundles[0], signedBlockBlobsBundles[1] = signedBlockBlobsBundles[1], signedBlockBlobsBundles[0]
	}

	// Broadcast the signed block and blobs
	if err := MultiPeerSignedBlockBlobBroadcast(spec, testPeers, s.SendBlobsFirst, signedBlockBlobsBundles...); err != nil {
		return false, errors.Wrap(err, "failed to broadcast signed beacon block")
	}

	return true, nil
}

/*
TODO: Refactor all of this

type ExtraBlobs struct {
	Default
	IncorrectKZGCommitment  bool `json:"incorrect_kzg_commitment"`
	IncorrectKZGProof       bool `json:"incorrect_kzg_proof"`
	IncorrectBlockRoot      bool `json:"incorrect_block_root"`
	IncorrectSignature      bool `json:"incorrect_signature"`
	DelayMilliseconds       int  `json:"delay_milliseconds"`
	BroadcastBlockFirst     bool `json:"broadcast_block_last"`
	BroadcastExtraBlobFirst bool `json:"broadcast_extra_blob_last"`
}

func (s ExtraBlobs) Name() string {
	return "Extra blobs"
}

func (s ExtraBlobs) Fields() map[string]interface{} {
	return map[string]interface{}{
		"incorrect_kzg_commitment":   s.IncorrectKZGCommitment,
		"incorrect_kzg_proof":        s.IncorrectKZGProof,
		"incorrect_block_root":       s.IncorrectBlockRoot,
		"incorrect_signature":        s.IncorrectSignature,
		"delay_milliseconds":         s.DelayMilliseconds,
		"broadcast_block_first":      s.BroadcastBlockFirst,
		"broadcast_extra_blob_first": s.BroadcastExtraBlobFirst,
	}
}

func FillSidecarWithRandomBlob(sidecar *deneb.BlobSidecar) error {
	blob, kgzCommitment, kzgProof, err := kzg.RandomBlob()
	if err != nil {
		return errors.Wrap(err, "failed to generate random blob")
	}
	sidecar.Blob = blob[:]
	copy(sidecar.KZGCommitment[:], kgzCommitment[:])
	copy(sidecar.KZGProof[:], kzgProof[:])
	return nil
}

func (s ExtraBlobs) Execute(
	spec *beacon_common.Spec,
	testPeers p2p.TestPeers,
	beaconBlockContents *deneb.BlockContents,
	beaconBlockDomain beacon_common.BLSDomain,
	validatorKey *keys.ValidatorKey,
	includeBlobRecord *common.BlobRecord,
	rejectBlobRecord *common.BlobRecord,
) (bool, error) {
	// Sign block
	signedBlockContents, err := SignBlockContents(spec, beaconBlockContents, beaconBlockDomain, validatorKey)
	if err != nil {
		return false, errors.Wrap(err, "failed to sign block")
	}
	signedBlobs, err := SignBlobs(blobSidecars, blobSidecarDomain, validatorKey)
	if err != nil {
		return false, errors.Wrap(err, "failed to sign blobs")
	}

	// Generate the extra blob sidecar
	extraBlobSidecar := &deneb.BlobSidecar{
		Slot:            beaconBlock.Slot,
		BlockParentRoot: beaconBlock.ParentRoot[:],
		ProposerIndex:   beaconBlock.ProposerIndex,
	}

	if s.IncorrectBlockRoot {
		extraBlobSidecar.BlockRoot = make([]byte, 32)
		rand.Read(extraBlobSidecar.BlockRoot)
	} else {
		blockRoot, err := beaconBlock.HashTreeRoot()
		if err != nil {
			return false, errors.Wrap(err, "failed to get block hash tree root")
		}
		extraBlobSidecar.BlockRoot = blockRoot[:]
	}

	if err := FillSidecarWithRandomBlob(extraBlobSidecar); err != nil {
		return false, errors.Wrap(err, "failed to fill extra blob sidecar")
	}

	if s.IncorrectKZGCommitment {
		fields := logrus.Fields{
			"correct": fmt.Sprintf("%x", extraBlobSidecar.KZGCommitment),
		}
		rand.Read(extraBlobSidecar.KZGCommitment)
		fields["corrupted"] = fmt.Sprintf("%x", extraBlobSidecar.KZGCommitment)
		logrus.WithFields(fields).Debug("Corrupted blob sidecar kzg commitment")
	}

	if s.IncorrectKZGProof {
		fields := logrus.Fields{
			"correct": fmt.Sprintf("%x", extraBlobSidecar.KZGProof),
		}
		rand.Read(extraBlobSidecar.KZGProof)
		fields["corrupted"] = fmt.Sprintf("%x", extraBlobSidecar.KZGProof)
		logrus.WithFields(fields).Debug("Corrupted blob sidecar kzg proof")
	}

	// Sign the blob
	signedExtraBlob, err := SignBlob(extraBlobSidecar, blobSidecarDomain, validatorKey)
	if err != nil {
		return false, errors.Wrap(err, "failed to sign extra blob")
	}

	if s.IncorrectSignature {
		fields := logrus.Fields{
			"correct": fmt.Sprintf("%x", signedExtraBlob.Signature),
		}
		rand.Read(signedExtraBlob.Signature)
		fields["corrupted"] = fmt.Sprintf("%x", signedExtraBlob.Signature)
		logrus.WithFields(fields).Debug("Corrupted blob sidecar signature")
	}

	logrus.WithFields(
		logrus.Fields{
			"blockRoot":       fmt.Sprintf("%x", extraBlobSidecar.BlockRoot),
			"blockParentRoot": fmt.Sprintf("%x", extraBlobSidecar.BlockParentRoot),
			"slot":            extraBlobSidecar.Slot,
			"proposerIndex":   extraBlobSidecar.ProposerIndex,
			"kzgCommitment":   fmt.Sprintf("%x", extraBlobSidecar.KZGCommitment),
			"kzgProof":        fmt.Sprintf("%x", extraBlobSidecar.KZGProof),
		},
	).Debug("Extra blob")

	if s.BroadcastBlockFirst {
		// Broadcast the block
		if err := testPeers.BroadcastSignedBeaconBlock(signedBlock); err != nil {
			return false, errors.Wrap(err, "failed to broadcast signed beacon block")
		}
	}

	if s.BroadcastExtraBlobFirst {
		// Broadcast the extra blob
		if err := testPeers.BroadcastSignedBlobSidecar(signedExtraBlob, nil); err != nil {
			return false, errors.Wrap(err, "failed to broadcast extra signed blob sidecar")
		}

		// Insert a delay before gossiping the blobs
		time.Sleep(time.Duration(s.DelayMilliseconds) * time.Millisecond)
	}

	// Broadcast the correct blobs
	if err := testPeers.BroadcastSignedBlobSidecars(signedBlobs); err != nil {
		return false, errors.Wrap(err, "failed to broadcast signed blob sidecar")
	}

	if !s.BroadcastExtraBlobFirst {
		// Insert a delay before gossiping the blobs
		time.Sleep(time.Duration(s.DelayMilliseconds) * time.Millisecond)

		// Broadcast the extra blob
		if err := testPeers.BroadcastSignedBlobSidecar(signedExtraBlob, nil); err != nil {
			return false, errors.Wrap(err, "failed to broadcast extra signed blob sidecar")
		}
	}

	if !s.BroadcastBlockFirst {
		// Broadcast the block
		if err := testPeers.BroadcastSignedBeaconBlock(signedBlock); err != nil {
			return false, errors.Wrap(err, "failed to broadcast signed beacon block")
		}
	}

	// Add the blobs to the must-include blob record
	includeBlobRecord.Add(beaconBlockContents.Block.Slot, blobSidecars...)

	// Add the extra blob to the must-reject blob record
	rejectBlobRecord.Add(beaconBlockContents.Block.Slot, extraBlobSidecar)

	return true, nil
}


type ConflictingBlobs struct {
	Default
	ConflictingBlobsCount       int  `json:"conflicting_blobs_count"`
	RandomConflictingBlobsCount bool `json:"random_conflicting_blobs_count"`
	AlternateBlobRecipients     bool `json:"alternate_blob_recipients"`
}

func (s ConflictingBlobs) Name() string {
	return "Conflicting blobs"
}

func (s ConflictingBlobs) Fields() map[string]interface{} {
	return map[string]interface{}{
		"conflicting_blobs_count":        s.ConflictingBlobsCount,
		"random_conflicting_blobs_count": s.RandomConflictingBlobsCount,
		"alternate_blob_recipients":      s.AlternateBlobRecipients,
	}
}

func (s ConflictingBlobs) GetTestPeerCount() int {
	// We are going to send two conflicting blobs through two different test p2p connections
	return 2
}

func (s ConflictingBlobs) GetConflictingBlobsCount() int {
	if s.RandomConflictingBlobsCount {
		return math_rand.Intn(MAX_BLOBS_PER_BLOCK-1) + 1
	}
	if s.ConflictingBlobsCount > 0 {
		return s.ConflictingBlobsCount
	}
	return 1
}

func (s ConflictingBlobs) Execute(
	spec *beacon_common.Spec,
	testPeers p2p.TestPeers,
	beaconBlockContents *deneb.BlockContents,
	beaconBlockDomain beacon_common.BLSDomain,
	validatorKey *keys.ValidatorKey,
	includeBlobRecord *common.BlobRecord,
	rejectBlobRecord *common.BlobRecord,
) (bool, error) {
	if len(testPeers) != 2 {
		return false, fmt.Errorf("expected 2 test p2p connections, got %d", len(testPeers))
	}

	// Sign block
	signedBlockContents, err := SignBlockContents(spec, beaconBlockContents, beaconBlockDomain, validatorKey)
	if err != nil {
		return false, errors.Wrap(err, "failed to sign block")
	}
	signedBlobs, err := SignBlobs(blobSidecars, blobSidecarDomain, validatorKey)
	if err != nil {
		return false, errors.Wrap(err, "failed to sign blobs")
	}

	// Generate the extra blob sidecars
	blockRoot, err := beaconBlock.HashTreeRoot()
	if err != nil {
		return false, errors.Wrap(err, "failed to get block hash tree root")
	}

	conflictingBlobsCount := s.GetConflictingBlobsCount()

	// Create the second list of sidecars
	secondBlobSidecarsLength := len(signedBlobs)
	if secondBlobSidecarsLength < conflictingBlobsCount {
		secondBlobSidecarsLength = conflictingBlobsCount
	}
	secondBlobSidecars := make([]*eth.SignedBlobSidecar, secondBlobSidecarsLength)

	for i := 0; i < secondBlobSidecarsLength; i++ {
		if i < conflictingBlobsCount {
			conflictingBlobSidecar := &deneb.BlobSidecar{
				BlockRoot:       blockRoot[:],
				Index:           uint64(i),
				Slot:            beaconBlock.Slot,
				BlockParentRoot: beaconBlock.ParentRoot[:],
				ProposerIndex:   beaconBlock.ProposerIndex,
			}

			if err := FillSidecarWithRandomBlob(conflictingBlobSidecar); err != nil {
				return false, errors.Wrap(err, "failed to fill extra blob sidecar")
			}
			// Sign the blob
			secondBlobSidecars[i], err = SignBlob(conflictingBlobSidecar, blobSidecarDomain, validatorKey)
			if err != nil {
				return false, errors.Wrap(err, "failed to sign extra blob")
			}

			// Add the blob to the must-reject blob record
			rejectBlobRecord.Add(beaconBlockContents.Block.Slot, conflictingBlobSidecar)
		} else {
			secondBlobSidecars[i] = signedBlobs[i]
		}
	}

	var signedBlobsBundles [][]*eth.SignedBlobSidecar
	if s.AlternateBlobRecipients && (beaconBlock.Slot%2 == 0) {
		signedBlobsBundles = [][]*eth.SignedBlobSidecar{secondBlobSidecars, signedBlobs}
	} else {
		signedBlobsBundles = [][]*eth.SignedBlobSidecar{signedBlobs, secondBlobSidecars}
	}
	if err := MultiPeerSignedBlobBroadcast(spec, testPeers, signedBlobsBundles); err != nil {
		return false, errors.Wrap(err, "failed to broadcast signed blob sidecars")
	}

	// Broadcast the block
	if err := testPeers.BroadcastSignedBeaconBlock(signedBlock); err != nil {
		return false, errors.Wrap(err, "failed to broadcast signed beacon block")
	}

	// Add the blobs to the must-include blob record
	includeBlobRecord.Add(beaconBlockContents.Block.Slot, blobSidecars...)

	return true, nil
}

// Send all correct blobs but swap the indexes of two blobs
// Split network: send the correct blobs to one half of the peers and the swapped blobs to
// the other half
type SwapBlobs struct {
	Default
	SplitNetwork bool `json:"split_network"`
}

func (s SwapBlobs) Name() string {
	return "Swap blobs"
}

func (s SwapBlobs) Fields() map[string]interface{} {
	return map[string]interface{}{
		"split_network": s.SplitNetwork,
	}
}

func (s SwapBlobs) GetTestPeerCount() int {
	// We are going to send conflicting blobs if the network is split
	if s.SplitNetwork {
		return 2
	}
	return 1
}

func (s SwapBlobs) ModifyBlobs(blobSidecars []*deneb.BlobSidecar) ([]*deneb.BlobSidecar, error) {
	modifiedBlobSidecars, err := CopyBlobSidecars(blobSidecars)
	if err != nil {
		return nil, errors.Wrap(err, "failed to copy blobs")
	}

	if len(blobSidecars) > 0 {
		// If we only have one blob, we can simply modify the index of this single blob
		if len(blobSidecars) == 1 {
			modifiedBlobSidecars[0].Index = 1
		} else {
			// Swap the indexes of two blobs
			firstIndex := math_rand.Intn(len(blobSidecars))
			secondIndex := math_rand.Intn(len(blobSidecars))
			for firstIndex == secondIndex {
				secondIndex = math_rand.Intn(len(blobSidecars))
			}
			modifiedBlobSidecars[firstIndex].Index = uint64(secondIndex)
			modifiedBlobSidecars[secondIndex].Index = uint64(firstIndex)

			// Swap the blobs (So they are sent in increased index order)
			tmpBlob := modifiedBlobSidecars[firstIndex]
			modifiedBlobSidecars[firstIndex] = modifiedBlobSidecars[secondIndex]
			modifiedBlobSidecars[secondIndex] = tmpBlob
		}
	}
	return modifiedBlobSidecars, nil
}

func (s SwapBlobs) Execute(
	spec *beacon_common.Spec,
	testPeers p2p.TestPeers,
	beaconBlockContents *deneb.BlockContents,
	beaconBlockDomain beacon_common.BLSDomain,
	validatorKey *keys.ValidatorKey,
	includeBlobRecord *common.BlobRecord,
	rejectBlobRecord *common.BlobRecord,
) (bool, error) {
	var (
		signedBlock          *eth.SignedBeaconBlockDeneb
		signedBlobs          []*eth.SignedBlobSidecar
		signedModifiedBlobs  []*eth.SignedBlobSidecar
		modifiedBlobSidecars []*deneb.BlobSidecar
		err                  error
	)

	if s.SplitNetwork && len(testPeers) != 2 {
		return false, fmt.Errorf("expected 2 test p2p connections, got %d", len(testPeers))
	}

	// Modify the blobs
	modifiedBlobSidecars, err = s.ModifyBlobs(blobSidecars)
	if err != nil {
		return false, errors.Wrap(err, "failed to modify blobs")
	}

	// Sign block
	signedBlock, err = SignBlock(spec, beaconBlock, beaconBlockDomain, validatorKey)
	if err != nil {
		return false, errors.Wrap(err, "failed to sign block")
	}
	if s.SplitNetwork {
		signedBlobs, err = SignBlobs(blobSidecars, blobSidecarDomain, validatorKey)
		if err != nil {
			return false, errors.Wrap(err, "failed to sign blobs")
		}
	}
	signedModifiedBlobs, err = SignBlobs(modifiedBlobSidecars, blobSidecarDomain, validatorKey)
	if err != nil {
		return false, errors.Wrap(err, "failed to sign modified blobs")
	}

	// Broadcast the blobs first for the test to make sense
	if s.SplitNetwork {
		if err := MultiPeerSignedBlobBroadcast(spec, testPeers, [][]*eth.SignedBlobSidecar{signedBlobs, signedModifiedBlobs}); err != nil {
			return false, errors.Wrap(err, "failed to broadcast signed blob sidecars")
		}
	} else {
		if err := testPeers.BroadcastSignedBlobSidecars(signedModifiedBlobs); err != nil {
			return false, errors.Wrap(err, "failed to broadcast signed blob sidecars")
		}
	}

	// Broadcast the block
	if err := testPeers.BroadcastSignedBeaconBlock(signedBlock); err != nil {
		return false, errors.Wrap(err, "failed to broadcast signed beacon block")
	}

	// Add the blobs to the records
	if s.SplitNetwork {
		// The signed blobs with the correct indexes do make their way into the network, so they must be present in the block
		includeBlobRecord.Add(beaconBlockContents.Block.Slot, blobSidecars...)
	} else {
		// Only the modified invalid blob sidecars make their way into the network, so they shouldn't be present in the block
		rejectBlobRecord.Add(beaconBlockContents.Block.Slot, modifiedBlobSidecars...)
	}

	return true, nil
}
*/
