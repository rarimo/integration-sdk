package zkverifier_kit

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	val "github.com/go-ozzo/ozzo-validation/v4"
	zkptypes "github.com/iden3/go-rapidsnark/types"
	zkpverifier "github.com/iden3/go-rapidsnark/verifier"
	circuit "github.com/rarimo/zkverifier-kit/circut"
	"gitlab.com/distributed_lab/logan/v3"
	"gitlab.com/distributed_lab/logan/v3/errors"
)

// predefined values and positions for public inputs in zero knowledge proof. It may change depending on the proof
// and the values that it reveals.
const (
	PubSignalNullifier      = 0
	pubSignalBirthDate      = 1
	pubSignalExpirationDate = 2
	pubSignalCitizenship    = 6
	pubSignalEventID        = 9
	pubSignalEventData      = 10
	pubSignalIdStateHash    = 11
	pubSignalSelector       = 12

	proofSelectorValue = "39"
)

// Verifier is a structure representing some instance for validation and verification zero knowledge proof
// generated by Rarimo system.
type Verifier struct {
	// verificationKey stores verification key content and uploads after constructor is called.
	verificationKey []byte
	// opts has fields that must be validated before proof verification.
	opts VerifyOptions
}

// NewPassportVerifier is a constructor to create a new Verifier instance. It takes optional amount of parameters that
// will be validated during Verifier.VerifyProof method. Also in content of the verification key is uploaded and
// stored.
func NewPassportVerifier(options ...VerifyOption) (Connector, error) {
	verifier := Verifier{
		opts: mergeOptions(options...),
	}

	var err error
	verifier.verificationKey, err = circuit.VerificationKey.ReadFile(circuit.VerificationKeyFileName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse", logan.F{
			"filename": circuit.VerificationKeyFileName,
		})
	}

	return &verifier, nil
}

// VerifyProof method is used for proof verification, it validates inputs and values that was initialised
// in NewVerifier function and then check ZKP with verification key downloaded at the same time using
// `github.com/iden3/go-rapidsnark/verifier` package.
func (v *Verifier) VerifyProof(proof zkptypes.ZKProof) error {
	if err := v.validate(proof); err != nil {
		return errors.Wrap(err, "failed to validate proof")
	}

	if err := zkpverifier.VerifyGroth16(proof, v.verificationKey); err != nil {
		return errors.Wrap(err, "failed to verify proof")
	}

	return nil
}

// VerifyExternalID is a method to check that externalID corresponds to the one that was set in options.
// ExternalID is an optional parameter that represents some user identifier to connect proof with, it
// has to be a hex encoded string (without 0x prefix) of SHA256 hash from the value that was passed in
// WithExternalID or SetExternalID methods.
func (v *Verifier) VerifyExternalID(externalID string) error {
	return val.Errors{
		"external_id": val.Validate(v.opts.externalID,
			val.Required,
			val.In(externalID),
		),
	}.Filter()
}

// validate is a helper method to validate public signals with values stored in opts field.
func (v *Verifier) validate(zkProof zkptypes.ZKProof) error {
	err := val.Errors{
		"zk_proof/proof":       val.Validate(zkProof.Proof, val.Required),
		"zk_proof/pub_signals": val.Validate(zkProof.PubSignals, val.Required, val.Length(14, 14)),
	}.Filter()
	if err != nil {
		return errors.Wrap(err, "failed to validate arguments")
	}

	return val.Errors{
		// Required fields to validate
		"pub_signals/nullifier":       val.Validate(zkProof.PubSignals[PubSignalNullifier], val.Required),
		"pub_signals/selector":        val.Validate(zkProof.PubSignals[pubSignalSelector], val.Required, val.In(proofSelectorValue)),
		"pub_signals/expiration_date": val.Validate(zkProof.PubSignals[pubSignalExpirationDate], val.Required, afterDate(time.Now().UTC())),
		"pub_signals/id_state_hash":   v.opts.rootVerifier.VerifyRoot(zkProof.PubSignals[pubSignalIdStateHash]),

		// Configurable fields
		"pub_signals/event_id": val.Validate(zkProof.PubSignals[pubSignalEventID], val.When(
			!val.IsEmpty(v.opts.eventID),
			val.Required,
			val.In(v.opts.eventID))),
		"pub_signals/birth_date": val.Validate(zkProof.PubSignals[pubSignalBirthDate], val.When(
			!val.IsEmpty(v.opts.age),
			val.Required,
			beforeDate(v.opts.age),
		)),
		"pub_signals/citizenship": val.Validate(mustDecodeInt(zkProof.PubSignals[pubSignalCitizenship]), val.When(
			!val.IsEmpty(v.opts.citizenships),
			val.Required,
			val.In(v.opts.citizenships...),
		)),
		"pub_signals/event_data": val.Validate(zkProof.PubSignals[pubSignalEventData], val.When(
			!val.IsEmpty(v.opts.address),
			val.Required,
			val.In(encodeInt(v.opts.address)),
		)),
	}.Filter()
}

// SetExternalID - helper method that can be used either to set empty external identifier or update existing one.
// This external ID is some value with which zero knowledge proof has to be associated with. This value has to be
// a raw one, then it will be hashed with SHA256 and stored.
func (v *Verifier) SetExternalID(externalID string) {
	idHash := sha256.Sum256([]byte(externalID))
	v.opts.externalID = hex.EncodeToString(idHash[:])
}
