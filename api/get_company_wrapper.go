package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"codeberg.org/cuducos/minha-receita/pkg/cnpjcanonical"
	"github.com/avast/retry-go/v4"
)

const (
	retries           = 13
	timeoutPerAttempt = 1 * time.Second
)

var errTimeout = errors.New("getCompany timed out")

// this wrapper avoids having the getCompany idle for too long, wrapping it in
// timeout and restarting it after that. Lookup uses canonical CNPJ (14 chars [0-9A-Z]).
func getCompany(db database, n string) (string, error) {
	canonical, err := cnpjcanonical.NormalizeCNPJCanonical(n)
	if err != nil {
		return "", fmt.Errorf("invalid CNPJ %q: %w", n, err)
	}
	var c string
	err = retry.Do(
		func() error {
			ctx, cancel := context.WithTimeout(context.Background(), timeoutPerAttempt)
			defer cancel()
			ch := make(chan error, 1)
			go func() {
				var err error
				c, err = db.GetCompany(canonical)
				ch <- err
			}()
			select {
			case <-ctx.Done():
				return errTimeout
			case err := <-ch:
				return err
			}
		},
		retry.Attempts(retries),
		retry.RetryIf(func(err error) bool {
			return err != nil && errors.Is(err, errTimeout)
		}),
	)
	if err != nil {
		return "", fmt.Errorf("error retrieving %s: %w", n, err)
	}
	return c, nil
}
