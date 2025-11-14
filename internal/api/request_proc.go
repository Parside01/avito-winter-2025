package api

import "github.com/labstack/echo/v4"

func ProcessRequest[T any](e echo.Context, req *T, steps ...func(echo.Context, *T) error) error {
	for _, step := range steps {
		if err := step(e, req); err != nil {
			return err
		}
	}
	return nil
}
