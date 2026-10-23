//go:build !windows

package winservice

import "errors"

func discoverPostgresTarget(string) (postgresTarget, error) {
	return postgresTarget{}, errors.New("tuning PostgreSQL hanya didukung pada Windows")
}

func systemPhysicalMemory() (uint64, error) {
	return 0, errors.New("deteksi RAM tuning PostgreSQL hanya didukung pada Windows")
}

func restartTunedPostgres(postgresTarget) error {
	return errors.New("restart PostgreSQL hanya didukung pada Windows")
}
