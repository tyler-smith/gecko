// (c) 2019-2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package keystore

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	"github.com/ava-labs/gecko/database/memdb"
	"github.com/ava-labs/gecko/ids"
	"github.com/ava-labs/gecko/utils/logging"
)

var (
	// strongPassword defines a password used for the following tests that
	// scores high enough to pass the password strength scoring system
	strongPassword = "N_+=_jJ;^(<;{4,:*m6CET}'&N;83FYK.wtNpwp-Jt"
)

func TestServiceListNoUsers(t *testing.T) {
	ks := Keystore{}
	ks.Initialize(logging.NoLog{}, memdb.New())

	reply := ListUsersReply{}
	if err := ks.ListUsers(nil, &ListUsersArgs{}, &reply); err != nil {
		t.Fatal(err)
	}
	if len(reply.Users) != 0 {
		t.Fatalf("No users should have been created yet")
	}
}

func TestServiceCreateUser(t *testing.T) {
	ks := Keystore{}
	ks.Initialize(logging.NoLog{}, memdb.New())

	{
		reply := CreateUserReply{}
		if err := ks.CreateUser(nil, &CreateUserArgs{
			Username: "bob",
			Password: strongPassword,
		}, &reply); err != nil {
			t.Fatal(err)
		}
		if !reply.Success {
			t.Fatalf("User should have been created successfully")
		}
	}

	{
		reply := ListUsersReply{}
		if err := ks.ListUsers(nil, &ListUsersArgs{}, &reply); err != nil {
			t.Fatal(err)
		}
		if len(reply.Users) != 1 {
			t.Fatalf("One user should have been created")
		}
		if user := reply.Users[0]; user != "bob" {
			t.Fatalf("'bob' should have been a user that was created")
		}
	}
}

// genStr returns a string of given length
func genStr(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:n]
}

// TestServiceCreateUserArgsChecks generates excessively long usernames or
// passwords to assure the santity checks on string length are not exceeded
func TestServiceCreateUserArgsCheck(t *testing.T) {
	ks := Keystore{}
	ks.Initialize(logging.NoLog{}, memdb.New())

	{
		reply := CreateUserReply{}
		err := ks.CreateUser(nil, &CreateUserArgs{
			Username: genStr(maxUserPassLen + 1),
			Password: strongPassword,
		}, &reply)

		if reply.Success || err != errUserPassMaxLength {
			t.Fatal("User was created when it should have been rejected due to too long a Username, err =", err)
		}
	}

	{
		reply := CreateUserReply{}
		err := ks.CreateUser(nil, &CreateUserArgs{
			Username: "shortuser",
			Password: genStr(maxUserPassLen + 1),
		}, &reply)

		if reply.Success || err != errUserPassMaxLength {
			t.Fatal("User was created when it should have been rejected due to too long a Password, err =", err)
		}
	}

	{
		reply := ListUsersReply{}
		if err := ks.ListUsers(nil, &ListUsersArgs{}, &reply); err != nil {
			t.Fatal(err)
		}

		if len(reply.Users) > 0 {
			t.Fatalf("A user exists when there should be none")
		}
	}
}

// TestServiceCreateUserWeakPassword tests creating a new user with a weak
// password to ensure the password strength check is working
func TestServiceCreateUserWeakPassword(t *testing.T) {
	ks := Keystore{}
	ks.Initialize(logging.NoLog{}, memdb.New())

	{
		reply := CreateUserReply{}
		err := ks.CreateUser(nil, &CreateUserArgs{
			Username: "bob",
			Password: "weak",
		}, &reply)

		if err != errWeakPassword {
			t.Error("Unexpected error occurred when testing weak password:", err)
		}

		if reply.Success {
			t.Fatal("User was created when it should have been rejected due to weak password")
		}
	}
}

func TestServiceCreateDuplicate(t *testing.T) {
	ks := Keystore{}
	ks.Initialize(logging.NoLog{}, memdb.New())

	{
		reply := CreateUserReply{}
		if err := ks.CreateUser(nil, &CreateUserArgs{
			Username: "bob",
			Password: strongPassword,
		}, &reply); err != nil {
			t.Fatal(err)
		}
		if !reply.Success {
			t.Fatalf("User should have been created successfully")
		}
	}

	{
		reply := CreateUserReply{}
		if err := ks.CreateUser(nil, &CreateUserArgs{
			Username: "bob",
			Password: strongPassword,
		}, &reply); err == nil {
			t.Fatalf("Should have errored due to the username already existing")
		}
	}
}

func TestServiceCreateUserNoName(t *testing.T) {
	ks := Keystore{}
	ks.Initialize(logging.NoLog{}, memdb.New())

	reply := CreateUserReply{}
	if err := ks.CreateUser(nil, &CreateUserArgs{
		Password: strongPassword,
	}, &reply); err == nil {
		t.Fatalf("Shouldn't have allowed empty username")
	}
}

func TestServiceUseBlockchainDB(t *testing.T) {
	ks := Keystore{}
	ks.Initialize(logging.NoLog{}, memdb.New())

	{
		reply := CreateUserReply{}
		if err := ks.CreateUser(nil, &CreateUserArgs{
			Username: "bob",
			Password: strongPassword,
		}, &reply); err != nil {
			t.Fatal(err)
		}
		if !reply.Success {
			t.Fatalf("User should have been created successfully")
		}
	}

	{
		db, err := ks.GetDatabase(ids.Empty, "bob", strongPassword)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Put([]byte("hello"), []byte("world")); err != nil {
			t.Fatal(err)
		}
	}

	{
		db, err := ks.GetDatabase(ids.Empty, "bob", strongPassword)
		if err != nil {
			t.Fatal(err)
		}
		if val, err := db.Get([]byte("hello")); err != nil {
			t.Fatal(err)
		} else if !bytes.Equal(val, []byte("world")) {
			t.Fatalf("Should have read '%s' from the db", "world")
		}
	}
}

func TestServiceExportImport(t *testing.T) {
	ks := Keystore{}
	ks.Initialize(logging.NoLog{}, memdb.New())

	{
		reply := CreateUserReply{}
		if err := ks.CreateUser(nil, &CreateUserArgs{
			Username: "bob",
			Password: strongPassword,
		}, &reply); err != nil {
			t.Fatal(err)
		}
		if !reply.Success {
			t.Fatalf("User should have been created successfully")
		}
	}

	{
		db, err := ks.GetDatabase(ids.Empty, "bob", strongPassword)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Put([]byte("hello"), []byte("world")); err != nil {
			t.Fatal(err)
		}
	}

	exportReply := ExportUserReply{}
	if err := ks.ExportUser(nil, &ExportUserArgs{
		Username: "bob",
		Password: strongPassword,
	}, &exportReply); err != nil {
		t.Fatal(err)
	}

	newKS := Keystore{}
	newKS.Initialize(logging.NoLog{}, memdb.New())

	{
		reply := ImportUserReply{}
		if err := newKS.ImportUser(nil, &ImportUserArgs{
			Username: "bob",
			Password: strongPassword,
			User:     exportReply.User,
		}, &reply); err != nil {
			t.Fatal(err)
		}
		if !reply.Success {
			t.Fatalf("User should have been imported successfully")
		}
	}

	{
		db, err := newKS.GetDatabase(ids.Empty, "bob", strongPassword)
		if err != nil {
			t.Fatal(err)
		}
		if val, err := db.Get([]byte("hello")); err != nil {
			t.Fatal(err)
		} else if !bytes.Equal(val, []byte("world")) {
			t.Fatalf("Should have read '%s' from the db", "world")
		}
	}
}
