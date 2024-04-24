package zkverifier_kit

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// VerifyOptions structure that stores all fields that may be validated before proof verification.
// All elements must be able to have "zero" value in order to skip it during validation. For
// structure validation `github.com/go-ozzo/ozzo-validation/v4` is used, so IsEmpty method has to
// work correct with each field in order to have supposed logic.
type VerifyOptions struct {
	// externalID - is an external identifier with which the proof is associated. This value has
	// to be in a raw format (e.g. email, rarimo address), because this library will hash this value
	// with SHA256 hashing and compare with the one tha will be passed during proof verification.
	externalID string
	// age - a minimal age required to proof some statement.
	age time.Time
	// citizenships - array of interfaces (for more convenient usage during validation) that stores
	// all citizenships that accepted in proof. Under the hood, it is a string of Alpha-3 county codes,
	// described in the ISO 3166 international standard.
	citizenships []interface{}
	// address - is any cosmos address for which proof was generated. It is stored in decoded form,
	// without prefix.
	address []byte
	// eventID - unique identifier associated with a specific event or interaction within
	// the protocol execution, may be used to keep track of various steps or actions, this
	// id is a string with a big integer in decimals format
	eventID string
}

// VerifyOption type alias for function that may add new values to VerifyOptions structure.
// It allows to create convenient methods With... that will add new value to the fields for
// that structure.
type VerifyOption func(*VerifyOptions)

// WithExternalID takes event identifier as a string, this is whatever the system wants to connect the proof
// with (e.g. email, phone number, incremental id, etc.)
func WithExternalID(identifier string) VerifyOption {
	return func(opts *VerifyOptions) {
		idHash := sha256.Sum256([]byte(identifier))
		opts.externalID = hex.EncodeToString(idHash[:])
	}
}

// WithAgeAbove adds new age check. It is an integer (e.g. 10, 18, 21) above which the person's
// age must be in proof.
func WithAgeAbove(age int) VerifyOption {
	return func(opts *VerifyOptions) {
		opts.age = time.Now().UTC().AddDate(-age, 0, 0)
	}
}

// WithCitizenships adds new available citizenship/s to prove that user is a resident of specified country.
// Function takes an arbitrary number of strings that consists from Alpha-3 county codes,
// described in the ISO 3166 international standard (e.g. "USA", "UKR", "TUR").
func WithCitizenships(citizenships ...string) VerifyOption {
	return func(opts *VerifyOptions) {
		opts.citizenships = make([]interface{}, len(citizenships))
		for i, ctz := range citizenships {
			opts.citizenships[i] = ctz
		}
	}
}

// WithAddress takes decoded address that must be validated in proof. It requires to have same format that is in
// proof public signals (for example: bech32 address decoded to base256 without human-readable part)
func WithAddress(address []byte) VerifyOption {
	return func(opts *VerifyOptions) {
		opts.address = address
	}
}

// WithEventID takes event identifier as a string that represents big number in a decimal format.
func WithEventID(identifier string) VerifyOption {
	return func(opts *VerifyOptions) {
		opts.eventID = identifier
	}
}

// mergeOptions function that collects all parameters together into one VerifyOptions structure that
// can be used further.
func mergeOptions(options ...VerifyOption) VerifyOptions {
	opts := VerifyOptions{}
	for _, opt := range options {
		opt(&opts)
	}
	return opts
}