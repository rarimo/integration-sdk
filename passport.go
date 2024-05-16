package zkverifier_kit

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	val "github.com/go-ozzo/ozzo-validation/v4"
	zkptypes "github.com/iden3/go-rapidsnark/types"
	zkpverifier "github.com/iden3/go-rapidsnark/verifier"
	"github.com/rarimo/zkverifier-kit/identity"
)

type PubSignal int

// predefined values and positions for public inputs in zero knowledge proof. It
// may change depending on the proof and the values that it reveals.
const (
	Nullifier                 PubSignal = 0
	Citizenship               PubSignal = 6
	EventID                   PubSignal = 9
	EventData                 PubSignal = 10
	IdStateRoot               PubSignal = 11
	Selector                  PubSignal = 12
	TimestampUpperBound       PubSignal = 14
	IdentityCounterUpperBound PubSignal = 16
	BirthdateUpperBound       PubSignal = 18
	ExpirationDateLowerBound  PubSignal = 19

	proofSelectorValue = "39"
)

var ErrVerificationKeyRequired = errors.New("verification key is required")

// Verifier is a structure representing some instance for validation and verification zero knowledge proof
// generated by Rarimo system.
type Verifier struct {
	// verificationKey stores verification key content
	verificationKey []byte
	// opts has fields that must be validated before proof verification.
	opts VerifyOptions
}

// NewPassportVerifier creates a new Verifier instance. VerificationKey is
// required to VerifyGroth16, usually you should just read it from file. Optional
// parameters will take part in proof verification on Verifier.VerifyProof call.
//
// If you provided WithVerificationKeyFile option, you can pass nil as the first arg.
func NewPassportVerifier(verificationKey []byte, options ...VerifyOption) (*Verifier, error) {
	verifier := Verifier{
		verificationKey: verificationKey,
		opts:            mergeOptions(VerifyOptions{}, options...),
	}

	file := verifier.opts.verificationKeyFile
	if file == "" {
		if len(verificationKey) == 0 {
			return nil, ErrVerificationKeyRequired
		}
		return &verifier, nil
	}

	var err error
	verifier.verificationKey, err = os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read verification key from file %q: %w", file, err)
	}

	return &verifier, nil
}

// VerifyProof method verifies iden3 ZK proof and checks public signals. The
// public signals to validate are defined in the VerifyOption list. Firstly, you
// pass initial values to verify in NewPassportVerifier. In case when custom
// values are required for different proofs, the options can be passed to
// VerifyProof, which override the initial ones.
func (v *Verifier) VerifyProof(proof zkptypes.ZKProof, options ...VerifyOption) error {
	v2 := Verifier{
		verificationKey: v.verificationKey,
		opts:            mergeOptions(v.opts, options...),
	}

	if err := v2.validateBase(proof); err != nil {
		return err
	}

	if err := zkpverifier.VerifyGroth16(proof, v.verificationKey); err != nil {
		return fmt.Errorf("groth16 verification failed: %w", err)
	}

	return nil
}

func (v *Verifier) validateBase(zkProof zkptypes.ZKProof) error {
	signals := zkProof.PubSignals

	err := val.Errors{
		"zk_proof/proof":       val.Validate(zkProof.Proof, val.Required),
		"zk_proof/pub_signals": val.Validate(signals, val.Required, val.Length(21, 21)),
	}.Filter()
	if err != nil {
		return err
	}

	err = v.opts.rootVerifier.VerifyRoot(signals[IdStateRoot])
	if errors.Is(err, identity.ErrContractCall) {
		return err
	}

	allowedBirthDate := time.Now().UTC().AddDate(-v.opts.age, 0, 0)
	all := val.Errors{
		"pub_signals/nullifier":                   val.Validate(signals[Nullifier], val.Required),
		"pub_signals/selector":                    val.Validate(signals[Selector], val.Required, val.In(proofSelectorValue)),
		"pub_signals/expiration_date_lower_bound": val.Validate(signals[ExpirationDateLowerBound], val.Required, afterDate(time.Now().UTC())),
		"pub_signals/id_state_hash":               err,
		"pub_signals/event_id":                    validateOnOptSet(signals[EventID], v.opts.eventID, val.In(v.opts.eventID)),
		// upper bound is a date: the earlier it is, the higher the age
		"pub_signals/birth_date_upper_bound": validateOnOptSet(signals[BirthdateUpperBound], v.opts.age, beforeDate(allowedBirthDate)),
		"pub_signals/citizenship":            validateOnOptSet(decodeInt(signals[Citizenship]), v.opts.citizenships, val.In(v.opts.citizenships...)),
		"pub_signals/event_data":             validateOnOptSet(signals[EventData], v.opts.eventDataRule, v.opts.eventDataRule),
	}

	for field, e := range v.validateIdentitiesInputs(signals) {
		all[field] = e
	}

	return all.Filter()
}

func (v *Verifier) validateIdentitiesInputs(signals []string) val.Errors {
	counter, err := strconv.ParseInt(signals[IdentityCounterUpperBound], 10, 64)
	if err != nil {
		return val.Errors{"pub_signals/identity_counter_upper_bound": err}
	}

	cErr := val.Validate(counter, val.When(
		v.opts.maxIdentitiesCount != -1,
		val.Required,
		val.Max(v.opts.maxIdentitiesCount),
	))
	tErr := validateOnOptSet(
		signals[TimestampUpperBound],
		v.opts.maxIdentityCreationTimestamp,
		beforeDate(v.opts.maxIdentityCreationTimestamp),
	)

	// OR logic: at least one of the signals should be valid
	if cErr != nil {
		return val.Errors{"pub_signals/timestamp_upper_bound": tErr}
	}
	if tErr != nil {
		return val.Errors{"pub_signals/identity_counter_upper_bound": cErr}
	}

	return nil
}
