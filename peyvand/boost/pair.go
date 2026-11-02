package boost

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// Pairing is numeric comparison, like Bluetooth: both sides show the same six digits and the user
// approves on the phone. The computer first commits to its public key, so a machine in the middle
// cannot pick keys that make both codes match (its chance is one in a million per attempt).
//
//	PC    -> phone  {"op":"pair","id","name","commit": sha256(pcPub)}
//	phone -> PC     {"id","name","pub": phonePub}
//	PC    -> phone  {"pub": pcPub}
//	   both show Code(pcPub, phonePub); the phone user approves or declines
//	phone -> PC     {"ok": true|false}
//
// The shared key is derived from the X25519 secret and never sent.

type pairHello struct {
	Op     string `json:"op"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	Commit string `json:"commit"`
}

type pairReply struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Pub  string `json:"pub"`
	Err  string `json:"err,omitempty"`
}

type pairReveal struct {
	Pub string `json:"pub"`
}

type pairResult struct {
	OK  bool   `json:"ok"`
	Err string `json:"err,omitempty"`
}

// Peer is the other side of a pairing.
type Peer struct {
	ID   string
	Name string
	Key  []byte // 32 bytes, shared by both sides
}

// ErrDeclined is returned when the phone user declines (or the request times out).
var ErrDeclined = errors.New("boost: pairing declined on the phone")

// Code is the six-digit number both screens show.
func Code(pcPub, phonePub []byte) string {
	h := sha256.New()
	h.Write([]byte("peyvand-code-v1"))
	h.Write(pcPub)
	h.Write(phonePub)
	n := binary.BigEndian.Uint32(h.Sum(nil)[:4]) % 1_000_000
	return fmt.Sprintf("%06d", n)
}

func deriveKey(shared, pcPub, phonePub []byte, pcID, phoneID string) []byte {
	h := sha256.New()
	h.Write([]byte("peyvand-key-v1"))
	h.Write(shared)
	h.Write(pcPub)
	h.Write(phonePub)
	h.Write([]byte(pcID + "\x00" + phoneID))
	return h.Sum(nil)
}

// PairClient runs the computer's side of pairing on rw (a fresh connection to the phone's
// service port). showCode is called with the code as soon as it is known; it returns false if the
// user cancelled on the computer, which aborts the pairing. On success the phone's identity and
// the shared key are returned.
func PairClient(rw io.ReadWriter, pcID, pcName string, showCode func(code, phoneName string) bool) (Peer, error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return Peer{}, err
	}
	pub := priv.PublicKey().Bytes()
	commit := sha256.Sum256(pub)
	if _, err := rw.Write([]byte{'J'}); err != nil {
		return Peer{}, err
	}
	if err := writeJSON(rw, pairHello{Op: "pair", ID: pcID, Name: pcName, Commit: hex.EncodeToString(commit[:])}); err != nil {
		return Peer{}, err
	}
	var rep pairReply
	if err := readJSON(rw, &rep); err != nil {
		return Peer{}, err
	}
	if rep.Err != "" {
		return Peer{}, errors.New("boost: " + rep.Err)
	}
	phonePub, err := hex.DecodeString(rep.Pub)
	if err != nil {
		return Peer{}, err
	}
	pk, err := ecdh.X25519().NewPublicKey(phonePub)
	if err != nil {
		return Peer{}, err
	}
	shared, err := priv.ECDH(pk)
	if err != nil {
		return Peer{}, err
	}
	if err := writeJSON(rw, pairReveal{Pub: hex.EncodeToString(pub)}); err != nil {
		return Peer{}, err
	}
	// The user compares codes while the phone waits for its user; cancelling here closes the
	// connection, which also clears the phone's prompt.
	if showCode != nil && !showCode(Code(pub, phonePub), rep.Name) {
		return Peer{}, errors.New("boost: pairing cancelled")
	}
	var res pairResult
	if err := readJSON(rw, &res); err != nil {
		return Peer{}, err
	}
	if !res.OK {
		return Peer{}, ErrDeclined
	}
	return Peer{ID: rep.ID, Name: rep.Name, Key: deriveKey(shared, pub, phonePub, pcID, rep.ID)}, nil
}

// pairServer is the phone's side, after the 'J' byte. approve shows the code and the computer's
// name and blocks until the user decides.
func pairServer(rw io.ReadWriter, phoneID, phoneName string, approve func(pcName, code string) bool) (Peer, error) {
	var hello pairHello
	if err := readJSON(rw, &hello); err != nil {
		return Peer{}, err
	}
	if hello.Op != "pair" || hello.ID == "" {
		return Peer{}, errors.New("boost: bad pairing request")
	}
	commit, err := hex.DecodeString(hello.Commit)
	if err != nil || len(commit) != sha256.Size {
		return Peer{}, errors.New("boost: bad commitment")
	}
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return Peer{}, err
	}
	pub := priv.PublicKey().Bytes()
	if err := writeJSON(rw, pairReply{ID: phoneID, Name: phoneName, Pub: hex.EncodeToString(pub)}); err != nil {
		return Peer{}, err
	}
	var rev pairReveal
	if err := readJSON(rw, &rev); err != nil {
		return Peer{}, err
	}
	pcPub, err := hex.DecodeString(rev.Pub)
	if err != nil {
		return Peer{}, err
	}
	if sum := sha256.Sum256(pcPub); !bytes.Equal(sum[:], commit) {
		writeJSON(rw, pairResult{Err: "commitment mismatch"})
		return Peer{}, errors.New("boost: commitment mismatch")
	}
	pk, err := ecdh.X25519().NewPublicKey(pcPub)
	if err != nil {
		return Peer{}, err
	}
	shared, err := priv.ECDH(pk)
	if err != nil {
		return Peer{}, err
	}
	ok := approve != nil && approve(hello.Name, Code(pcPub, pub))
	if err := writeJSON(rw, pairResult{OK: ok}); err != nil {
		return Peer{}, err
	}
	if !ok {
		return Peer{}, ErrDeclined
	}
	return Peer{ID: hello.ID, Name: hello.Name, Key: deriveKey(shared, pcPub, pub, hello.ID, phoneID)}, nil
}
