package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"os"
)

const backupMagic = "ZQZTBK16"
const backupChunk = 4 << 20

// Each frame authenticates its position. An authenticated empty final frame
// distinguishes a complete archive from a truncated but otherwise valid prefix.
func backupCrypt(input io.Reader, output io.Writer, key []byte, encrypt bool) error {
	header := make([]byte, 40)
	if encrypt {
		copy(header, backupMagic)
		if _, err := rand.Read(header[8:]); err != nil {
			return err
		}
		if _, err := output.Write(header); err != nil {
			return err
		}
	} else {
		if _, err := io.ReadFull(input, header); err != nil {
			return errors.New("invalid_backup")
		}
		if !bytes.Equal(header[:8], []byte(backupMagic)) {
			return errors.New("invalid_backup")
		}
	}
	kdf := hmac.New(sha256.New, key)
	kdf.Write(header)
	gcm, err := encryptionGCM(kdf.Sum(nil))
	if err != nil {
		return err
	}
	counter := uint32(0)
	buffer := make([]byte, backupChunk)
	nonce := make([]byte, 12)
	copy(nonce, header[8:16])
	aad := make([]byte, 44)
	copy(aad, header)
	for {
		binary.BigEndian.PutUint32(nonce[8:], counter)
		binary.BigEndian.PutUint32(aad[40:], counter)
		if encrypt {
			n, readErr := io.ReadFull(input, buffer)
			if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
				return readErr
			}
			sealed := gcm.Seal(nil, nonce, buffer[:n], aad)
			var length [4]byte
			binary.BigEndian.PutUint32(length[:], uint32(len(sealed)))
			if _, err = output.Write(length[:]); err != nil {
				return err
			}
			if _, err = output.Write(sealed); err != nil {
				return err
			}
			if n == 0 {
				return nil
			}
		} else {
			var length [4]byte
			if _, err = io.ReadFull(input, length[:]); err != nil {
				return errors.New("invalid_backup_truncated")
			}
			size := binary.BigEndian.Uint32(length[:])
			if size < uint32(gcm.Overhead()) || size > backupChunk+uint32(gcm.Overhead()) {
				return errors.New("invalid_backup_frame")
			}
			sealed := make([]byte, size)
			if _, err = io.ReadFull(input, sealed); err != nil {
				return errors.New("invalid_backup_truncated")
			}
			plain, e := gcm.Open(nil, nonce, sealed, aad)
			if e != nil {
				return errors.New("backup_key_or_checksum")
			}
			if len(plain) == 0 {
				var extra [1]byte
				n, e := input.Read(extra[:])
				if n != 0 || e != io.EOF {
					return errors.New("invalid_backup_trailing_data")
				}
				return nil
			}
			if _, err = output.Write(plain); err != nil {
				return err
			}
		}
		if counter == ^uint32(0) {
			return errors.New("invalid_backup_size")
		}
		counter++
	}
}
func cryptBackupFile(source, target string, key []byte, encrypt bool) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	err = backupCrypt(in, out, key, encrypt)
	if err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(target)
	}
	return err
}
