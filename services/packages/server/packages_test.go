/* Copyright (c) 2019 Snowflake Inc. All rights reserved.

   Licensed under the Apache License, Version 2.0 (the
   "License"); you may not use this file except in compliance
   with the License.  You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing,
   software distributed under the License is distributed on an
   "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
   KIND, either express or implied.  See the License for the
   specific language governing permissions and limitations
   under the License.
*/

package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/testing/protocmp"

	pb "github.com/Snowflake-Labs/sansshell/services/packages"
	"github.com/Snowflake-Labs/sansshell/testing/testutil"
)

var (
	bufSize = 1024 * 1024
	lis     *bufconn.Listener
	conn    *grpc.ClientConn
)

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func TestMain(m *testing.M) {
	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()
	lfs := &server{}
	lfs.Register(s)
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
	defer s.GracefulStop()

	os.Exit(m.Run())
}

func TestInstall(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("Failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	client := pb.NewPackagesClient(conn)

	testdataInput := "This is output we expect to see\n\nMore output\n"
	savedGenerateInstall := generateInstall
	var cmdLine string
	generateInstall = func(i *pb.InstallRequest) ([]string, error) {
		// Capture what was generated so we can validate it.
		out, err := savedGenerateInstall(i)
		if err != nil {
			return nil, err
		}
		cmdLine = strings.Join(out, " ")
		return []string{testutil.ResolvePath(t, "echo"), "-n", testdataInput}, nil
	}
	t.Cleanup(func() { generateInstall = savedGenerateInstall })

	// Test 0: Bunch of permutations for invalid input.
	for _, tc := range []struct {
		name string
		req  *pb.InstallRequest
	}{
		{
			name: "bad package system",
			req: &pb.InstallRequest{
				PackageSystem: pb.PackageSystem_PACKAGE_SYSTEM_YUM + 99,
				Name:          "package",
				Version:       "1.2.3",
			},
		},
		{
			name: "no name given",
			req: &pb.InstallRequest{
				Version: "1.2.3",
			},
		},
		{
			name: "no version given",
			req: &pb.InstallRequest{
				Name: "package",
			},
		},
		{
			name: "bad name - starts with a dash",
			req: &pb.InstallRequest{
				Name:    "-package",
				Version: "1.2.3",
			},
		},
		{
			name: "bad version - starts with a dash",
			req: &pb.InstallRequest{
				Name:    "package",
				Version: "-1.2.3",
			},
		},
		{
			name: "invalid characters in name",
			req: &pb.InstallRequest{
				Name:    "package && rm -rf /",
				Version: "1.2.3",
			},
		},
		{
			name: "invalid characters in version",
			req: &pb.InstallRequest{
				Name:    "package",
				Version: "1.2.3 && rm -rf /",
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.Install(ctx, tc.req)
			testutil.FatalOnNoErr(fmt.Sprintf("%v - resp %v", tc.name, resp), err, t)
			t.Logf("%s: %v", tc.name, err)
		})
	}

	req := &pb.InstallRequest{
		Name:        "package",
		Version:     "1.2.3",
		Repo:        "somerepo",
		DisableRepo: "otherrepo",
	}

	savedYumBin := YumBin
	t.Cleanup(func() {
		YumBin = savedYumBin
	})

	// Test 1: Should fail on a blank yum
	YumBin = ""
	_, err = client.Install(ctx, req)
	testutil.FatalOnNoErr("clean install request", err, t)

	// Test 2: A clean install. Validate we got expected output back.
	// This is assuming yum based installs for testing command builder.
	YumBin = "yum"
	wantCmdLine := fmt.Sprintf("%s install-nevra -y --disablerepo=otherrepo --enablerepo=somerepo package-1.2.3", YumBin)

	resp, err := client.Install(ctx, req)
	testutil.FatalOnErr("clean install request", err, t)
	if got, want := resp.DebugOutput, testdataInput; got != want {
		t.Fatalf("Output from clean install differs. Got:\n%q\nWant:\n%q", got, want)
	}
	if got, want := cmdLine, wantCmdLine; got != want {
		t.Fatalf("command lines differ. Got %q Want %q", got, want)
	}
	t.Logf("clean install response: %+v", resp)

	// Test 3: Permutations on bad commands/output.
	for _, tc := range []struct {
		name     string
		generate func(*pb.InstallRequest) ([]string, error)
	}{
		{
			name: "bad command",
			generate: func(*pb.InstallRequest) ([]string, error) {
				return []string{"/non-existant-binary"}, nil
			},
		},
		{
			name: "bad path",
			generate: func(*pb.InstallRequest) ([]string, error) {
				return []string{"non-existant-binary"}, nil
			},
		},
		{
			name: "bad exit code",
			generate: func(*pb.InstallRequest) ([]string, error) {
				return []string{testutil.ResolvePath(t, "false")}, nil
			},
		},
	} {
		tc := tc
		saveGenerate := generateInstall
		t.Run(tc.name, func(t *testing.T) {
			generateInstall = tc.generate
			resp, err := client.Install(ctx, req)
			testutil.FatalOnNoErr(fmt.Sprintf("%v - resp %v", tc.name, resp), err, t)
			t.Log(err)
		})
		generateInstall = saveGenerate
	}
}

func TestUpdate(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("Failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	client := pb.NewPackagesClient(conn)

	testdataInput := "This is output we expect to see\n\nMore output\n"
	savedGenerateValidate := generateValidate
	savedGenerateUpdate := generateUpdate
	var cmdLine, validateCmdLine string
	generateValidate = func(u *pb.UpdateRequest) ([]string, error) {
		// Capture what was generated so we can validate it.
		out, err := savedGenerateValidate(u)
		if err != nil {
			return nil, err
		}
		validateCmdLine = strings.Join(out, " ")
		return []string{testutil.ResolvePath(t, "echo"), "-n", testdataInput}, nil
	}
	badValidate := func(u *pb.UpdateRequest) ([]string, error) {
		// Capture what was generated so we can validate it.
		out, err := savedGenerateValidate(u)
		if err != nil {
			return nil, err
		}
		validateCmdLine = strings.Join(out, " ")
		return []string{testutil.ResolvePath(t, "false")}, nil
	}

	generateUpdate = func(u *pb.UpdateRequest) ([]string, error) {
		// Capture what was generated so we can validate it.
		out, err := savedGenerateUpdate(u)
		if err != nil {
			return nil, err
		}
		cmdLine = strings.Join(out, " ")
		return []string{testutil.ResolvePath(t, "echo"), "-n", testdataInput}, nil
	}
	t.Cleanup(func() {
		generateValidate = savedGenerateValidate
		generateUpdate = savedGenerateUpdate
	})

	// Test 0: Bunch of permutations for invalid input.
	for _, tc := range []struct {
		name string
		req  *pb.UpdateRequest
	}{
		{
			name: "bad package system",
			req: &pb.UpdateRequest{
				PackageSystem: pb.PackageSystem_PACKAGE_SYSTEM_YUM + 99,
				Name:          "package",
				OldVersion:    "0:1-1.2.3",
				NewVersion:    "0:1-4.5.6",
			},
		},
		{
			name: "bad old version - nevra",
			req: &pb.UpdateRequest{
				PackageSystem: pb.PackageSystem_PACKAGE_SYSTEM_YUM,
				Name:          "package",
				OldVersion:    "1.2.3",
				NewVersion:    "0:1-4.5.6",
			},
		},
		{
			name: "bad new version - nevra",
			req: &pb.UpdateRequest{
				PackageSystem: pb.PackageSystem_PACKAGE_SYSTEM_YUM,
				Name:          "package",
				OldVersion:    "0:1-1.2.3",
				NewVersion:    "4.5.6",
			},
		},
		{
			name: "no name given",
			req: &pb.UpdateRequest{
				OldVersion: "1.2.3",
				NewVersion: "4.5.6",
			},
		},
		{
			name: "no old version given",
			req: &pb.UpdateRequest{
				Name:       "package",
				NewVersion: "4.5.6",
			},
		},
		{
			name: "no new version given",
			req: &pb.UpdateRequest{
				Name:       "package",
				OldVersion: "1.2.3",
			},
		},
		{
			name: "bad name - starts with a dash",
			req: &pb.UpdateRequest{
				Name:       "-package",
				OldVersion: "1.2.3",
				NewVersion: "4.5.6",
			},
		},
		{
			name: "bad old version - starts with a dash",
			req: &pb.UpdateRequest{
				Name:       "package",
				OldVersion: "-1.2.3",
				NewVersion: "4.5.6",
			},
		},
		{
			name: "bad new version - starts with a dash",
			req: &pb.UpdateRequest{
				Name:       "package",
				OldVersion: "1.2.3",
				NewVersion: "-4.5.6",
			},
		},
		{
			name: "invalid characters in name",
			req: &pb.UpdateRequest{
				Name:       "package && rm -rf /",
				OldVersion: "1.2.3",
				NewVersion: "4.5.6",
			},
		},
		{
			name: "invalid characters in old version",
			req: &pb.UpdateRequest{
				Name:       "package",
				OldVersion: "1.2.3 && rm -rf /",
				NewVersion: "4.5.6",
			},
		},
		{
			name: "invalid characters in new version",
			req: &pb.UpdateRequest{
				Name:       "package",
				OldVersion: "1.2.3",
				NewVersion: "4.5.6 && rm -rf /",
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.Update(ctx, tc.req)
			testutil.FatalOnNoErr(fmt.Sprintf("%v - resp %v", tc.name, resp), err, t)
			t.Logf("%s: %v", tc.name, err)
		})
	}

	req := &pb.UpdateRequest{
		Name:        "package",
		OldVersion:  "0:1-1.2.3",
		NewVersion:  "0:1-4.5.6",
		Repo:        "somerepo",
		DisableRepo: "otherrepo",
	}

	savedYumBin := YumBin
	t.Cleanup(func() {
		YumBin = savedYumBin
	})
	// Test 1: A clean install. Validate we got expected output back.

	// This is assuming yum based installs for testing command builder.
	YumBin = "yum"
	wantValidateCmdLine := fmt.Sprintf("%s list installed package-0:1-1.2.3", YumBin)
	wantCmdLine := fmt.Sprintf("%s update-to -y --disablerepo=otherrepo --enablerepo=somerepo package-0:1-4.5.6", YumBin)

	resp, err := client.Update(ctx, req)
	testutil.FatalOnErr("clean update request", err, t)
	if got, want := resp.DebugOutput, testdataInput; got != want {
		t.Fatalf("Output from clean update differs. Got:\n%q\nWant:\n%q", got, want)
	}
	if got, want := validateCmdLine, wantValidateCmdLine; got != want {
		t.Fatalf("validate command lines differ. Got %q Want %q", got, want)
	}
	if got, want := cmdLine, wantCmdLine; got != want {
		t.Fatalf("command lines differ. Got %q Want %q", got, want)
	}
	t.Logf("clean install response: %+v", resp)

	// Test 2: Validation fails:
	save := generateValidate
	generateValidate = badValidate
	_, err = client.Update(ctx, req)
	testutil.FatalOnNoErr("validate should fail", err, t)
	t.Log(err)
	generateValidate = save

	// Test 3: Permutations on bad commands/output.
	for _, tc := range []struct {
		name     string
		generate func(*pb.UpdateRequest) ([]string, error)
		validate func(*pb.UpdateRequest) ([]string, error)
	}{
		{
			name: "bad command",
			generate: func(*pb.UpdateRequest) ([]string, error) {
				return []string{"/non-existant-binary"}, nil
			},
		},
		{
			name: "bad path - validate",
			validate: func(*pb.UpdateRequest) ([]string, error) {
				return []string{"bad path"}, nil
			},
		},
		{
			name: "bad path - update",
			generate: func(*pb.UpdateRequest) ([]string, error) {
				return []string{"bad path"}, nil
			},
		},
		{
			name: "bad exit code",
			generate: func(*pb.UpdateRequest) ([]string, error) {
				return []string{testutil.ResolvePath(t, "false")}, nil
			},
		},
	} {
		tc := tc
		saveGenerate := generateUpdate
		saveValidate := generateValidate
		t.Run(tc.name, func(t *testing.T) {
			if tc.generate != nil {
				generateUpdate = tc.generate
			}
			if tc.validate != nil {
				generateValidate = tc.validate
			}
			resp, err := client.Update(ctx, req)
			testutil.FatalOnNoErr(fmt.Sprintf("%v - resp %v", tc.name, resp), err, t)
			t.Log(err)
		})
		generateUpdate = saveGenerate
		generateValidate = saveValidate
	}
}
func TestListInstalled(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("Failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	client := pb.NewPackagesClient(conn)

	// Setup for feeding in test data for further tests.
	testdataInput := "./testdata/yum-installed.out"
	testdataInputBad := "./testdata/yum-installed-bad.out"
	testdataInputBad2 := "./testdata/yum-installed-bad2.out"
	testdataInputBad3 := "./testdata/yum-installed-bad3.out"
	testdataGolden := "./testdata/yum-installed.textproto"

	savedGenerateListInstalled := generateListInstalled
	var cmdLine string
	generateListInstalled = func(p pb.PackageSystem) ([]string, error) {
		// Capture what was generated so we can validate it.
		out, err := savedGenerateListInstalled(p)
		if err != nil {
			return nil, err
		}
		cmdLine = strings.Join(out, " ")
		return []string{testutil.ResolvePath(t, "cat"), testdataInput}, nil
	}
	t.Cleanup(func() {
		generateListInstalled = savedGenerateListInstalled
	})

	input, err := os.ReadFile(testdataGolden)
	testutil.FatalOnErr(fmt.Sprintf("can't read testdata golden from %s", testdataGolden), err, t)

	testdata := &pb.ListInstalledReply{}
	err = prototext.Unmarshal(input, testdata)
	testutil.FatalOnErr("Can't unmarshall test data", err, t)

	// Be able to sort the overall entries in a response
	sortEntries := protocmp.SortRepeated(func(i *pb.PackageInfo, j *pb.PackageInfo) bool {
		return i.Name < j.Name && i.Version < j.Version
	})

	// Test 0: Specify a bad package system and get an error.
	resp, err := client.ListInstalled(ctx, &pb.ListInstalledRequest{
		PackageSystem: pb.PackageSystem_PACKAGE_SYSTEM_YUM + 99,
	})
	if err == nil {
		t.Fatalf("didn't get an error as expected for a bad package enum. Instead got %+v", resp)
	}
	t.Log(err)

	savedYumBin := YumBin
	t.Cleanup(func() {
		YumBin = savedYumBin
	})
	// Test 1: No options. Should pick yum w/o error and give back our list.

	// This is assuming yum based installs for testing command builder.
	YumBin = "yum"
	wantCmdLine := fmt.Sprintf("%s list installed", YumBin)

	resp, err = client.ListInstalled(ctx, &pb.ListInstalledRequest{})
	testutil.FatalOnErr("basic package list request", err, t)

	testutil.DiffErr("basic package list request", resp, testdata, t, sortEntries)

	if got, want := cmdLine, wantCmdLine; got != want {
		t.Fatalf("command lines differ. Got %q Want %q", got, want)
	}

	// Test 2: Specify yum this time.
	resp, err = client.ListInstalled(ctx, &pb.ListInstalledRequest{
		PackageSystem: pb.PackageSystem_PACKAGE_SYSTEM_YUM,
	})
	testutil.FatalOnErr("basic package list request", err, t)

	testutil.DiffErr("basic package list request yum", resp, testdata, t, sortEntries)

	// Test 3: Now try with bad input. Should error out.
	for _, b := range []string{testdataInputBad, testdataInputBad2, testdataInputBad3} {
		generateListInstalled = func(pb.PackageSystem) ([]string, error) {
			return []string{testutil.ResolvePath(t, "cat"), b}, nil
		}
		resp, err = client.ListInstalled(ctx, &pb.ListInstalledRequest{
			PackageSystem: pb.PackageSystem_PACKAGE_SYSTEM_YUM,
		})
		testutil.FatalOnNoErr(fmt.Sprintf("bad input - resp %v", resp), err, t)
		t.Log(err)
	}

	// Test 4: Permutations of bad commands/exit codes, stderr output.
	for _, tc := range []struct {
		name     string
		generate func(pb.PackageSystem) ([]string, error)
	}{
		{
			name: "non-existant binary",
			generate: func(pb.PackageSystem) ([]string, error) {
				return []string{"/non-existant-binary"}, nil
			},
		},
		{
			name: "bad path",
			generate: func(pb.PackageSystem) ([]string, error) {
				return []string{"non-existant-binary"}, nil
			},
		},
		{
			name: "non-zero exit",
			generate: func(pb.PackageSystem) ([]string, error) {
				return []string{testutil.ResolvePath(t, "false")}, nil
			},
		},
	} {
		tc := tc
		saveGenerate := generateListInstalled
		t.Run(tc.name, func(t *testing.T) {
			generateListInstalled = tc.generate
			resp, err = client.ListInstalled(ctx, &pb.ListInstalledRequest{})
			testutil.FatalOnNoErr(fmt.Sprintf("%v - resp %v", tc.name, resp), err, t)
			t.Log(err)
		})
		generateListInstalled = saveGenerate
	}
}

func TestRepoList(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("Failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	client := pb.NewPackagesClient(conn)

	// Setup for feeding in test data for further tests.
	testdataInput := "./testdata/yum-repolist.out"
	testdataGolden := "./testdata/yum-repolist.textproto"

	savedGenerateRepoList := generateRepoList
	var cmdLine string
	generateRepoList = func(p pb.PackageSystem) ([]string, error) {
		// Capture what was generated so we can validate it.
		out, err := savedGenerateRepoList(p)
		if err != nil {
			return nil, err
		}
		cmdLine = strings.Join(out, " ")
		return []string{testutil.ResolvePath(t, "cat"), testdataInput}, nil
	}
	t.Cleanup(func() { generateRepoList = savedGenerateRepoList })

	input, err := os.ReadFile(testdataGolden)
	testutil.FatalOnErr(fmt.Sprintf("Can't read testdata golden %s", testdataGolden), err, t)

	testdata := &pb.RepoListReply{}
	err = prototext.Unmarshal(input, testdata)
	testutil.FatalOnErr("can't unmarshal test data", err, t)

	// Be able to sort the overall entries in a response
	sortEntries := protocmp.SortRepeated(func(i *pb.Repo, j *pb.Repo) bool {
		return i.Id < j.Id && i.Name < j.Name
	})

	// Test 0: Specify a bad package system and get an error.
	resp, err := client.RepoList(ctx, &pb.RepoListRequest{
		PackageSystem: pb.PackageSystem_PACKAGE_SYSTEM_YUM + 99,
	})
	testutil.FatalOnNoErr(fmt.Sprintf("bad package enum - resp %v", resp), err, t)
	t.Log(err)

	savedYumBin := YumBin
	t.Cleanup(func() {
		YumBin = savedYumBin
	})
	// Test 1: No options. Should pick yum w/o error and give back our list.

	// This is assuming yum based installs for testing command builder.
	YumBin = "yum"
	wantCmdLine := fmt.Sprintf("%s repoinfo all", YumBin)

	resp, err = client.RepoList(ctx, &pb.RepoListRequest{})
	testutil.FatalOnErr("basic repo list request", err, t)

	testutil.DiffErr("no options repo list", resp, testdata, t, sortEntries)

	if got, want := cmdLine, wantCmdLine; got != want {
		t.Fatalf("command lines differ. Got %q Want %q", got, want)
	}
	// Test 2: Specify yum this time.
	resp, err = client.RepoList(ctx, &pb.RepoListRequest{
		PackageSystem: pb.PackageSystem_PACKAGE_SYSTEM_YUM,
	})
	testutil.FatalOnErr("basic repo list request", err, t)

	testutil.DiffErr("repo list yum", resp, testdata, t, sortEntries)

	// Test 3: Permutations of bad commands/exit codes, stderr output.
	for _, tc := range []struct {
		name     string
		generate func(pb.PackageSystem) ([]string, error)
	}{
		{
			name: "non-existant binary",
			generate: func(pb.PackageSystem) ([]string, error) {
				return []string{"/non-existant-binary"}, nil
			},
		},
		{
			name: "bad path",
			generate: func(pb.PackageSystem) ([]string, error) {
				return []string{"non-existant-binary"}, nil
			},
		},
		{
			name: "non-zero exit",
			generate: func(pb.PackageSystem) ([]string, error) {
				return []string{testutil.ResolvePath(t, "false")}, nil
			},
		},
	} {
		tc := tc
		saveGenerate := generateRepoList
		t.Run(tc.name, func(t *testing.T) {
			generateRepoList = tc.generate
			resp, err = client.RepoList(ctx, &pb.RepoListRequest{})
			testutil.FatalOnNoErr(fmt.Sprintf("%v - resp %v", tc.name, resp), err, t)
			t.Log(err)
		})
		generateRepoList = saveGenerate
	}
}

func TestCleanup(t *testing.T) {
	var err error
	ctx := context.Background()
	conn, err = grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	testutil.FatalOnErr("Failed to dial bufnet", err, t)
	t.Cleanup(func() { conn.Close() })

	savedGenerateCleanup := generateCleanup
	t.Cleanup(func() {
		generateCleanup = savedGenerateCleanup
	})
	var cmdLine string

	for _, tc := range []struct {
		name     string
		pkg      pb.PackageSystem
		cleanup  string
		generate func(pb.PackageSystem) ([]string, error)
		wantErr  bool
		output   string
	}{
		{
			name:    "No package system",
			pkg:     pb.PackageSystem_PACKAGE_SYSTEM_UNKNOWN,
			cleanup: "yum-complete-transaction",
			generate: func(p pb.PackageSystem) ([]string, error) {
				// Capture what was generated so we can validate it.
				out, err := savedGenerateCleanup(p)
				if err != nil {
					return nil, err
				}
				cmdLine = strings.Join(out, " ")
				return []string{testutil.ResolvePath(t, "echo"), "output"}, nil
			},
			output: "output\n",
		},
		{
			name:    "yum package system",
			pkg:     pb.PackageSystem_PACKAGE_SYSTEM_YUM,
			cleanup: "yum-complete-transaction",
			generate: func(p pb.PackageSystem) ([]string, error) {
				// Capture what was generated so we can validate it.
				out, err := savedGenerateCleanup(p)
				if err != nil {
					return nil, err
				}
				cmdLine = strings.Join(out, " ")
				return []string{testutil.ResolvePath(t, "echo"), "output"}, nil
			},
			output: "output\n",
		},
		{
			name:     "missing cleanup binary",
			pkg:      pb.PackageSystem_PACKAGE_SYSTEM_YUM,
			generate: savedGenerateCleanup,
			wantErr:  true,
		},
		{
			name:     "bad package system",
			pkg:      pb.PackageSystem_PACKAGE_SYSTEM_YUM + 99,
			cleanup:  "yum-complete-transaction",
			generate: savedGenerateCleanup,
			wantErr:  true,
		},
		{
			name: "non-existant binary",
			generate: func(pb.PackageSystem) ([]string, error) {
				return []string{"/non-existant-binary"}, nil
			},
			wantErr: true,
		},
		{
			name: "bad path",
			generate: func(pb.PackageSystem) ([]string, error) {
				return []string{"non-existant-binary"}, nil
			},
			wantErr: true,
		},
		{
			name: "non-zero exit",
			generate: func(pb.PackageSystem) ([]string, error) {
				return []string{testutil.ResolvePath(t, "false")}, nil
			},
			wantErr: true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			saveGenerateCleanup := generateCleanup
			savedYumCleanup := YumCleanup
			t.Cleanup(func() {
				generateCleanup = saveGenerateCleanup
				YumCleanup = savedYumCleanup
			})
			generateCleanup = tc.generate
			YumCleanup = tc.cleanup

			client := pb.NewPackagesClient(conn)

			resp, err := client.Cleanup(ctx, &pb.CleanupRequest{
				PackageSystem: tc.pkg,
			})
			t.Log(err)
			testutil.WantErr(tc.name, err, tc.wantErr, t)
			if !tc.wantErr {
				wantCmdLine := fmt.Sprintf("%s --cleanup-only", YumCleanup)
				if got, want := cmdLine, wantCmdLine; got != want {
					t.Fatalf("command lines differ. Got %q Want %q", got, want)
				}
				if got, want := resp.DebugOutput, tc.output; got != want {
					t.Fatalf("command output differs. Got %q want %q", got, want)
				}

			}
		})
	}
}
