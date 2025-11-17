package mocks

import (
	"context"
	"github.com/stretchr/testify/mock"
)

type MockTransactor struct {
	mock.Mock
}

func (m *MockTransactor) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	args := m.Called(ctx, fn)

	if len(args) > 0 && args.Get(0) != nil {
		return args.Error(0)
	}

	return fn(ctx)
}

func (m *MockTransactor) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
