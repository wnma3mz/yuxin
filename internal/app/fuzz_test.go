package app

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func FuzzParseAmountRoundTrip(f *testing.F) {
	for _, seed := range []string{"0", "16800", "20w", "20万", "200k", "200,000.25", "-1", "NaN", "Inf", "", "1e309"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > 64<<10 {
			t.Skip()
		}
		amount, err := parseAmount(value)
		if err != nil {
			return
		}
		if math.IsNaN(amount) || math.IsInf(amount, 0) {
			t.Fatalf("parseAmount(%q) returned non-finite %v", value, amount)
		}
		roundTrip, err := parseAmount(configNumber(amount))
		if err != nil || roundTrip != amount {
			t.Fatalf("amount round trip: input=%q amount=%v got=%v err=%v", value, amount, roundTrip, err)
		}
	})
}

func FuzzLoadConfigCanonicalRoundTrip(f *testing.F) {
	f.Add(append([]byte(nil), bundledDefaultConfig...))
	f.Add([]byte("version = 1\n[salary]\namount = \"20w\"\n"))
	f.Add([]byte("[privacy]\nhide_amounts = true\nhide_retirement_date = true\n"))
	f.Add([]byte("[[assets]]\nname = \"a#b\"\nkind = \"deposit\"\nbalance = \"1\"\n"))
	f.Add([]byte("[salary]\namount = \"1\"\namount = \"2\"\n"))
	f.Add([]byte{0, 1, 2, 0xff, '\n'})

	directory := f.TempDir()
	source := filepath.Join(directory, "fuzz-input.toml")
	firstCanonical := filepath.Join(directory, "first.toml")
	secondCanonical := filepath.Join(directory, "second.toml")
	f.Fuzz(func(t *testing.T, content []byte) {
		if len(content) > 256<<10 {
			t.Skip()
		}
		if err := os.WriteFile(source, content, 0o600); err != nil {
			t.Fatal(err)
		}
		config, err := loadConfig(source)
		if err != nil {
			return
		}
		if err := saveConfig(config, firstCanonical); err != nil {
			t.Fatalf("accepted config cannot be saved: %v", err)
		}
		reloaded, err := loadConfig(firstCanonical)
		if err != nil {
			t.Fatalf("canonical config cannot be loaded: %v", err)
		}
		if err := saveConfig(reloaded, secondCanonical); err != nil {
			t.Fatalf("reloaded config cannot be saved: %v", err)
		}
		first, err := os.ReadFile(firstCanonical)
		if err != nil {
			t.Fatal(err)
		}
		second, err := os.ReadFile(secondCanonical)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("canonical serialization is unstable:\nfirst=%q\nsecond=%q", first, second)
		}
	})
}

func FuzzVerifyChecksumBoundaries(f *testing.F) {
	f.Add([]byte("release archive"), []byte("bad checksum"), "yuxin.zip")
	f.Add([]byte{}, []byte{}, "")
	f.Add([]byte{0, 1, 2}, []byte("00  wrong.zip\n"), "right.zip")
	f.Fuzz(func(t *testing.T, content, suppliedChecksum []byte, filenameSeed string) {
		if len(content) > 1<<20 || len(suppliedChecksum) > 64<<10 || len(filenameSeed) > 4<<10 {
			t.Skip()
		}
		// Arbitrary checksum input must be rejected or accepted without panicking.
		_ = verifyChecksum(content, suppliedChecksum, filenameSeed)

		nameDigest := sha256.Sum256([]byte(filenameSeed))
		safeName := hex.EncodeToString(nameDigest[:]) + ".zip"
		contentDigest := sha256.Sum256(content)
		valid := []byte(fmt.Sprintf("%x  %s\n", contentDigest, safeName))
		if err := verifyChecksum(content, valid, safeName); err != nil {
			t.Fatalf("generated checksum rejected: %v", err)
		}
		changed := append([]byte(nil), content...)
		if len(changed) == 0 {
			changed = []byte{1}
		} else {
			changed[0] ^= 0xff
		}
		if err := verifyChecksum(changed, valid, safeName); err == nil {
			t.Fatal("changed content passed the original checksum")
		}
	})
}

func FuzzExtractExecutableBoundaries(f *testing.F) {
	f.Add([]byte("not a zip"), []byte("binary"))
	f.Add([]byte{}, []byte{})
	seedArchive, err := executableArchive("nested/yuxin", []byte("seed"))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seedArchive, []byte("payload"))

	f.Fuzz(func(t *testing.T, arbitraryArchive, payload []byte) {
		if len(arbitraryArchive) > 1<<20 || len(payload) > 64<<10 {
			t.Skip()
		}
		// Keep arbitrary compressed inputs bounded even if they expand heavily.
		_ = extractExecutable(arbitraryArchive, "yuxin", &boundedFuzzWriter{remaining: 2 << 20})

		archive, err := executableArchive("release/nested/yuxin", payload)
		if err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		if err := extractExecutable(archive, "yuxin", &output); err != nil {
			t.Fatalf("valid executable archive rejected: %v", err)
		}
		if !bytes.Equal(output.Bytes(), payload) {
			t.Fatalf("extracted payload mismatch: got %d bytes, want %d", output.Len(), len(payload))
		}
	})
}

func executableArchive(name string, content []byte) ([]byte, error) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	entry, err := writer.Create(name)
	if err != nil {
		return nil, err
	}
	if _, err := entry.Write(content); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return archive.Bytes(), nil
}

var errFuzzOutputLimit = errors.New("fuzz output limit reached")

type boundedFuzzWriter struct {
	remaining int
}

func (writer *boundedFuzzWriter) Write(content []byte) (int, error) {
	if len(content) > writer.remaining {
		return 0, errFuzzOutputLimit
	}
	writer.remaining -= len(content)
	return len(content), nil
}

var _ io.Writer = (*boundedFuzzWriter)(nil)
