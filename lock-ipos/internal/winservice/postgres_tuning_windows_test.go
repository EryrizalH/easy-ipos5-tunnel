//go:build windows

package winservice

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestSystemPhysicalMemoryWindowsAPI(t *testing.T) {
	ram, err := systemPhysicalMemory()
	if err != nil {
		t.Fatalf("GlobalMemoryStatusEx failed: %v", err)
	}
	if ram == 0 {
		t.Fatal("GlobalMemoryStatusEx returned no physical memory")
	}
}

func TestSelectedPostgresServiceClassification(t *testing.T) {
	for _, test := range []struct {
		command string
		want    bool
	}{
		{`"C:\Program Files\PG\bin\pg_ctl.exe" runservice -D C:\data`, true},
		{`"c:/program files/pg/bin/POSTGRES.EXE" -D C:\data`, true},
		// Recognize a selected executable even if its arguments are malformed;
		// discovery must fail for this candidate, not silently skip it.
		{`"C:\Program Files\PG\bin\pg_ctl.exe" runservice -D`, true},
		{`"C:\Program Files\Other PG\bin\pg_ctl.exe" runservice -D C:\data`, false},
		{`"C:\Program Files\PG\bin\unrelated.exe"`, false},
		{`C:\Windows\System32\svchost.exe -k netsvcs`, false},
		{"", false},
	} {
		if got := isSelectedPostgresService(test.command, `C:\Program Files\PG\bin`); got != test.want {
			t.Errorf("candidate %q = %v, want %v", test.command, got, test.want)
		}
	}
}

func TestPostgresServiceArgumentsPreserveSelectedClusterAndOptions(t *testing.T) {
	executable, args, err := postgresServiceArguments(`"C:\Program Files\PostgreSQL\9.5\bin\pg_ctl.exe" runservice -N "postgresql-x64-9.5" -D "D:\IPOS Database\data" -w -o "-p 5445 -c config_file=\"D:\IPOS Database\postgresql.conf\""`)
	if err != nil {
		t.Fatal(err)
	}
	if executable != `C:\Program Files\PostgreSQL\9.5\bin\pg_ctl.exe` {
		t.Fatalf("unexpected executable %q", executable)
	}
	want := []string{"-D", `D:\IPOS Database\data`, "-p", "5445", "-c", `config_file=D:\IPOS Database\postgresql.conf`}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v; want %#v", args, want)
	}
}

func TestPostgresServiceArgumentsAttachedDataDirectory(t *testing.T) {
	for _, command := range []string{
		`C:\PG\bin\pg_ctl.exe runservice --pgdata=C:\PG\data --options="-p 5444"`,
		`C:\PG\bin\pg_ctl.exe runservice -DC:\PG\data -o "-p 5444"`,
		`C:\PG\bin\pg_ctl.exe runservice -DC:\PG\data -o"-p 5444"`,
		`C:\PG\bin\postgres.exe -D C:\PG\data -p 5444`,
	} {
		_, args, err := postgresServiceArguments(command)
		if err != nil {
			t.Errorf("%s: %v", command, err)
			continue
		}
		if directory, err := postgresArgumentDataDir(args); err != nil || directory != `C:\PG\data` {
			t.Errorf("directory = %q, %v", directory, err)
		}
	}
}

func TestPostgresServiceArgumentsRejectUnclearIdentity(t *testing.T) {
	for _, command := range []string{
		`pg_ctl.exe runservice -D C:\PG\data`,
		`C:\PG\bin\other.exe -D C:\PG\data`,
		`C:\PG\bin\pg_ctl.exe runservice`,
		`C:\PG\bin\pg_ctl.exe runservice -D data`,
		`C:\PG\bin\pg_ctl.exe runservice -D`,
		`C:\PG\bin\pg_ctl.exe runservice -D C:\PG\data -o`,
		`C:\PG\bin\pg_ctl.exe runservice -D C:\PG\data -o "-D C:\PG\other"`,
		`C:\PG\bin\postgres.exe --single -D C:\PG\data`,
		`C:\PG\bin\postgres.exe -D C:\PG\data -C config_file`,
		"C:\\PG\\bin\\pg_ctl.exe\x00 -D C:\\PG\\data",
	} {
		if _, _, err := postgresServiceArguments(command); err == nil {
			t.Errorf("accepted unclear service: %q", command)
		}
	}
}

func TestParsePostmasterIdentity(t *testing.T) {
	data := "4123\r\nD:/IPOS Database/data\r\n1761100000\r\n5444\r\n"
	pid, port, err := parsePostmasterIdentity([]byte(data), `D:\IPOS Database\data`)
	if err != nil || pid != 4123 || port != 5444 {
		t.Fatalf("identity = %d, %d, %v", pid, port, err)
	}
	for _, invalid := range []string{
		strings.Replace(data, "4123", "0", 1),
		strings.Replace(data, "4123", "-4123", 1),
		strings.Replace(data, "5444", "5432", 1),
		strings.Replace(data, "IPOS Database", "other", 1),
		"4123\nD:/IPOS Database/data\n",
	} {
		if _, _, err := parsePostmasterIdentity([]byte(invalid), `D:\IPOS Database\data`); err == nil {
			t.Errorf("accepted invalid postmaster identity: %q", invalid)
		}
	}
}

func TestMatchPostgresConfiguredPortRejectsPendingOperatorChange(t *testing.T) {
	for _, port := range []int{5444, 5445} {
		if err := matchPostgresConfiguredPort(fmt.Sprint(port), port); err != nil {
			t.Fatalf("unchanged port rejected: %v", err)
		}
	}
	for _, value := range []string{"5445", "5432", "", "invalid"} {
		if err := matchPostgresConfiguredPort(value, 5444); err == nil {
			t.Errorf("accepted mismatched/invalid configured port %q", value)
		}
	}
}

func TestSamePostgresPathRequiresAbsoluteAndNormalizesWindowsPath(t *testing.T) {
	if !samePostgresPath(`C:\PG\data\..\data`, `c:/pg/data`) {
		t.Fatal("equivalent absolute Windows paths were not matched")
	}
	if samePostgresPath(`data`, `data`) || samePostgresPath(`C:\PG\data`, `C:\PG\data2`) {
		t.Fatal("relative or different paths were matched")
	}
}
