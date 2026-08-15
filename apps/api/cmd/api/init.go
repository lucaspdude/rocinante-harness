package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucaspdude/rocinante-harness/apps/api/internal/auth"
	"github.com/lucaspdude/rocinante-harness/apps/api/internal/storage"
)

func initShare(shareDir string, noEncryption bool, passphraseEnv string) error {
	if err := os.MkdirAll(shareDir, 0o700); err != nil {
		return fmt.Errorf("mkdir share-dir: %w", err)
	}
	ed25519Path := filepath.Join(shareDir, ".ed25519")
	dbPath := filepath.Join(shareDir, "roc-harness.db")

	db, err := storage.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := storage.ApplyMigrations(db); err != nil {
		return err
	}

	if _, err := os.Stat(ed25519Path); err == nil {
		return fmt.Errorf(".ed25519 already exists at %s", ed25519Path)
	}

	sk, pk, err := auth.NewKeyPair()
	if err != nil {
		return err
	}

	passphrase := ""
	if !noEncryption {
		passphrase, err = readPassphrase(passphraseEnv)
		if err != nil {
			return err
		}
	}

	if noEncryption {
		if err := auth.SaveKeyFilePlaintext(ed25519Path, sk, pk); err != nil {
			return err
		}
		log.Printf("wrote plaintext key to %s", ed25519Path)
	} else {
		if err := auth.SaveKeyFileEncrypted(ed25519Path, sk, pk, passphrase, auth.DefaultKDFParams); err != nil {
			return err
		}
		backupPath := filepath.Join(shareDir, ".ed25519.bak")
		_ = auth.SaveKeyFileEncrypted(backupPath, sk, pk, passphrase, auth.DefaultKDFParams)
		log.Printf("wrote encrypted key to %s (backup: %s)", ed25519Path, backupPath)
	}
	_ = pk
	return nil
}

func readPassphrase(envName string) (string, error) {
	if envName != "" {
		if v := os.Getenv(envName); v != "" {
			return v, nil
		}
		return "", fmt.Errorf("env %s is empty", envName)
	}
	// A single bufio reader over os.Stdin so the kernel-level byte
	// stream is consumed sequentially; multiple readers would
	// buffer past the first line and lose subsequent lines.
	reader := bufio.NewReader(os.Stdin)
	fmt.Fprint(os.Stderr, "Create passphrase: ")
	first, err := readLineFrom(reader)
	if err != nil {
		return "", err
	}
	fmt.Fprint(os.Stderr, "Confirm passphrase: ")
	second, err := readLineFrom(reader)
	if err != nil {
		return "", err
	}
	if first != second {
		return "", fmt.Errorf("passphrases do not match")
	}
	if strings.TrimSpace(first) == "" {
		return "", fmt.Errorf("empty passphrase")
	}
	return first, nil
}

func readLineFrom(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
