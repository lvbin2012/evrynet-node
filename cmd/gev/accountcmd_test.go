// Copyright 2016 The evrynet-node Authors
// This file is part of evrynet-node.
//
// evrynet-node is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// evrynet-node is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with evrynet-node. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"io/ioutil"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cespare/cp"
)

// These tests are 'smoke tests' for the account related
// subcommands and flags.
//
// For most tests, the test files from package accounts
// are copied into a temporary keystore directory.

func tmpDatadirWithKeystore(t *testing.T) string {
	datadir := tmpdir(t)
	keystore := filepath.Join(datadir, "keystore")
	source := filepath.Join("..", "..", "accounts", "keystore", "testdata", "keystore")
	if err := cp.CopyAll(keystore, source); err != nil {
		t.Fatal(err)
	}
	return datadir
}

func TestAccountListEmpty(t *testing.T) {
	gev := runGev(t, "account", "list")
	gev.ExpectExit()
}

func TestAccountList(t *testing.T) {
	datadir := tmpDatadirWithKeystore(t)
	gev := runGev(t, "account", "list", "--datadir", datadir)
	defer gev.ExpectExit()
	if runtime.GOOS == "windows" {
		gev.Expect(`
Account #0: {EUjCujBMGzMuzdu6SChYq3gFrKrHVZXnZG} keystore://{{.Datadir}}\keystore\UTC--2016-03-22T12-57-55.920751759Z--EUjCujBMGzMuzdu6SChYq3gFrKrHVZXnZG
Account #1: {EfSBBjvr9A4L8W8GTyEbhNrKYLbgSorRzB} keystore://{{.Datadir}}\keystore\aaa
Account #2: {ELrewT2HwDPKCFbAW2A2ttbKnFwFZNfKXp} keystore://{{.Datadir}}\keystore\zzz
`)
	} else {
		gev.Expect(`
Account #0: {EUjCujBMGzMuzdu6SChYq3gFrKrHVZXnZG} keystore://{{.Datadir}}/keystore/UTC--2016-03-22T12-57-55.920751759Z--EUjCujBMGzMuzdu6SChYq3gFrKrHVZXnZG
Account #1: {EfSBBjvr9A4L8W8GTyEbhNrKYLbgSorRzB} keystore://{{.Datadir}}/keystore/aaa
Account #2: {ELrewT2HwDPKCFbAW2A2ttbKnFwFZNfKXp} keystore://{{.Datadir}}/keystore/zzz
`)
	}
}

func TestAccountNew(t *testing.T) {
	gev := runGev(t, "account", "new", "--lightkdf")
	defer gev.ExpectExit()
	gev.Expect(`
Your new account is locked with a password. Please give a password. Do not forget this password.
!! Unsupported terminal, password will be echoed.
Passphrase: {{.InputLine "foobar"}}
Repeat passphrase: {{.InputLine "foobar"}}

Your new key was generated
`)
	gev.ExpectRegexp(`
Public address of the key:   E[1-9a-km-zA-HJ-NP-Z]{33}
Path of the secret key file: .*UTC--.+--E[1-9a-km-zA-HJ-NP-Z]{33}

- You can share your public address with anyone. Others need it to interact with you.
- You must NEVER share the secret key with anyone! The key controls access to your funds!
- You must BACKUP your key file! Without the key, it's impossible to access account funds!
- You must REMEMBER your password! Without the password, it's impossible to decrypt the key!
`)
}

func TestAccountNewBadRepeat(t *testing.T) {
	gev := runGev(t, "account", "new", "--lightkdf")
	defer gev.ExpectExit()
	gev.Expect(`
Your new account is locked with a password. Please give a password. Do not forget this password.
!! Unsupported terminal, password will be echoed.
Passphrase: {{.InputLine "something"}}
Repeat passphrase: {{.InputLine "something else"}}
Fatal: Passphrases do not match
`)
}

func TestAccountUpdate(t *testing.T) {
	datadir := tmpDatadirWithKeystore(t)
	gev := runGev(t, "account", "update",
		"--datadir", datadir, "--lightkdf",
		"EfSBBjvr9A4L8W8GTyEbhNrKYLbgSorRzB")
	defer gev.ExpectExit()
	gev.Expect(`
Unlocking account EfSBBjvr9A4L8W8GTyEbhNrKYLbgSorRzB | Attempt 1/3
!! Unsupported terminal, password will be echoed.
Passphrase: {{.InputLine "foobar"}}
Please give a new password. Do not forget this password.
Passphrase: {{.InputLine "foobar2"}}
Repeat passphrase: {{.InputLine "foobar2"}}
`)
}

func TestWalletImport(t *testing.T) {
	gev := runGev(t, "wallet", "import", "--lightkdf", "testdata/guswallet.json")
	defer gev.ExpectExit()
	gev.Expect(`
!! Unsupported terminal, password will be echoed.
Passphrase: {{.InputLine "foo"}}
Address: {EcWgX4DxhvAcNwzzo7jQ6SweutC7B4wiAc}
`)

	files, err := ioutil.ReadDir(filepath.Join(gev.Datadir, "keystore"))
	if len(files) != 1 {
		t.Errorf("expected one key file in keystore directory, found %d files (error: %v)", len(files), err)
	}
}

func TestWalletImportBadPassword(t *testing.T) {
	gev := runGev(t, "wallet", "import", "--lightkdf", "testdata/guswallet.json")
	defer gev.ExpectExit()
	gev.Expect(`
!! Unsupported terminal, password will be echoed.
Passphrase: {{.InputLine "wrong"}}
Fatal: could not decrypt key with given passphrase
`)
}

func TestUnlockFlag(t *testing.T) {
	datadir := tmpDatadirWithKeystore(t)
	gev := runGev(t,
		"--datadir", datadir, "--nat", "none", "--nodiscover", "--maxpeers", "0", "--port", "0",
		"--unlock", "EfSBBjvr9A4L8W8GTyEbhNrKYLbgSorRzB",
		"js", "testdata/empty.js")
	gev.Expect(`
Unlocking account EfSBBjvr9A4L8W8GTyEbhNrKYLbgSorRzB | Attempt 1/3
!! Unsupported terminal, password will be echoed.
Passphrase: {{.InputLine "foobar"}}
`)
	gev.ExpectExit()

	wantMessages := []string{
		"Unlocked account",
		"=EfSBBjvr9A4L8W8GTyEbhNrKYLbgSorRzB",
	}
	for _, m := range wantMessages {
		if !strings.Contains(gev.StderrText(), m) {
			t.Errorf("stderr text does not contain %q", m)
		}
	}
}

func TestUnlockFlagWrongPassword(t *testing.T) {
	datadir := tmpDatadirWithKeystore(t)
	gev := runGev(t,
		"--datadir", datadir, "--nat", "none", "--nodiscover", "--maxpeers", "0", "--port", "0",
		"--unlock", "EfSBBjvr9A4L8W8GTyEbhNrKYLbgSorRzB")
	defer gev.ExpectExit()
	gev.Expect(`
Unlocking account EfSBBjvr9A4L8W8GTyEbhNrKYLbgSorRzB | Attempt 1/3
!! Unsupported terminal, password will be echoed.
Passphrase: {{.InputLine "wrong1"}}
Unlocking account EfSBBjvr9A4L8W8GTyEbhNrKYLbgSorRzB | Attempt 2/3
Passphrase: {{.InputLine "wrong2"}}
Unlocking account EfSBBjvr9A4L8W8GTyEbhNrKYLbgSorRzB | Attempt 3/3
Passphrase: {{.InputLine "wrong3"}}
Fatal: Failed to unlock account EfSBBjvr9A4L8W8GTyEbhNrKYLbgSorRzB (could not decrypt key with given passphrase)
`)
}

// https://github.com/Evrynetlabs/evrynet-node/issues/1785
func TestUnlockFlagMultiIndex(t *testing.T) {
	datadir := tmpDatadirWithKeystore(t)
	gev := runGev(t,
		"--datadir", datadir, "--nat", "none", "--nodiscover", "--maxpeers", "0", "--port", "0",
		"--unlock", "0,2",
		"js", "testdata/empty.js")
	gev.Expect(`
Unlocking account 0 | Attempt 1/3
!! Unsupported terminal, password will be echoed.
Passphrase: {{.InputLine "foobar"}}
Unlocking account 2 | Attempt 1/3
Passphrase: {{.InputLine "foobar"}}
`)
	gev.ExpectExit()

	wantMessages := []string{
		"Unlocked account",
		"=EUjCujBMGzMuzdu6SChYq3gFrKrHVZXnZG",
		"=ELrewT2HwDPKCFbAW2A2ttbKnFwFZNfKXp",
	}
	for _, m := range wantMessages {
		if !strings.Contains(gev.StderrText(), m) {
			t.Errorf("stderr text does not contain %q", m)
		}
	}
}

func TestUnlockFlagPasswordFile(t *testing.T) {
	datadir := tmpDatadirWithKeystore(t)
	gev := runGev(t,
		"--datadir", datadir, "--nat", "none", "--nodiscover", "--maxpeers", "0", "--port", "0",
		"--password", "testdata/passwords.txt", "--unlock", "0,2",
		"js", "testdata/empty.js")
	gev.ExpectExit()

	wantMessages := []string{
		"Unlocked account",
		"=EUjCujBMGzMuzdu6SChYq3gFrKrHVZXnZG",
		"=ELrewT2HwDPKCFbAW2A2ttbKnFwFZNfKXp",
	}
	for _, m := range wantMessages {
		if !strings.Contains(gev.StderrText(), m) {
			t.Errorf("stderr text does not contain %q", m)
		}
	}
}

func TestUnlockFlagPasswordFileWrongPassword(t *testing.T) {
	datadir := tmpDatadirWithKeystore(t)
	gev := runGev(t,
		"--datadir", datadir, "--nat", "none", "--nodiscover", "--maxpeers", "0", "--port", "0",
		"--password", "testdata/wrong-passwords.txt", "--unlock", "0,2")
	defer gev.ExpectExit()
	gev.Expect(`
Fatal: Failed to unlock account 0 (could not decrypt key with given passphrase)
`)
}

func TestUnlockFlagAmbiguous(t *testing.T) {
	store := filepath.Join("..", "..", "accounts", "keystore", "testdata", "dupes")
	gev := runGev(t,
		"--keystore", store, "--nat", "none", "--nodiscover", "--maxpeers", "0", "--port", "0",
		"--unlock", "EfSBBjvr9A4L8W8GTyEbhNrKYLbgSorRzB",
		"js", "testdata/empty.js")
	defer gev.ExpectExit()

	// Helper for the expect template, returns absolute keystore path.
	gev.SetTemplateFunc("keypath", func(file string) string {
		abs, _ := filepath.Abs(filepath.Join(store, file))
		return abs
	})
	gev.Expect(`
Unlocking account EfSBBjvr9A4L8W8GTyEbhNrKYLbgSorRzB | Attempt 1/3
!! Unsupported terminal, password will be echoed.
Passphrase: {{.InputLine "foobar"}}
Multiple key files exist for address EfSBBjvr9A4L8W8GTyEbhNrKYLbgSorRzB:
   keystore://{{keypath "1"}}
   keystore://{{keypath "2"}}
Testing your passphrase against all of them...
Your passphrase unlocked keystore://{{keypath "1"}}
In order to avoid this warning, you need to remove the following duplicate key files:
   keystore://{{keypath "2"}}
`)
	gev.ExpectExit()

	wantMessages := []string{
		"Unlocked account",
		"=EfSBBjvr9A4L8W8GTyEbhNrKYLbgSorRzB",
	}
	for _, m := range wantMessages {
		if !strings.Contains(gev.StderrText(), m) {
			t.Errorf("stderr text does not contain %q", m)
		}
	}
}

func TestUnlockFlagAmbiguousWrongPassword(t *testing.T) {
	store := filepath.Join("..", "..", "accounts", "keystore", "testdata", "dupes")
	gev := runGev(t,
		"--keystore", store, "--nat", "none", "--nodiscover", "--maxpeers", "0", "--port", "0",
		"--unlock", "EfSBBjvr9A4L8W8GTyEbhNrKYLbgSorRzB")
	defer gev.ExpectExit()

	// Helper for the expect template, returns absolute keystore path.
	gev.SetTemplateFunc("keypath", func(file string) string {
		abs, _ := filepath.Abs(filepath.Join(store, file))
		return abs
	})
	gev.Expect(`
Unlocking account EfSBBjvr9A4L8W8GTyEbhNrKYLbgSorRzB | Attempt 1/3
!! Unsupported terminal, password will be echoed.
Passphrase: {{.InputLine "wrong"}}
Multiple key files exist for address EfSBBjvr9A4L8W8GTyEbhNrKYLbgSorRzB:
   keystore://{{keypath "1"}}
   keystore://{{keypath "2"}}
Testing your passphrase against all of them...
Fatal: None of the listed files could be unlocked.
`)
	gev.ExpectExit()
}
