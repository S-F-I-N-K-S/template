package get_test

import (
	context "context"
	reflect "reflect"

	trip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
	uuid "github.com/google/uuid"
	gomock "go.uber.org/mock/gomock"
)

type MocktripReader struct {
	ctrl     *gomock.Controller
	recorder *MocktripReaderMockRecorder
	isgomock struct{}
}

type MocktripReaderMockRecorder struct {
	mock *MocktripReader
}

func NewMocktripReader(ctrl *gomock.Controller) *MocktripReader {
	mock := &MocktripReader{ctrl: ctrl}
	mock.recorder = &MocktripReaderMockRecorder{mock}
	return mock
}

func (m *MocktripReader) EXPECT() *MocktripReaderMockRecorder {
	return m.recorder
}

func (m *MocktripReader) GetByID(ctx context.Context, id uuid.UUID) (trip.Trip, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetByID", ctx, id)
	ret0, _ := ret[0].(trip.Trip)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MocktripReaderMockRecorder) GetByID(ctx, id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetByID", reflect.TypeOf((*MocktripReader)(nil).GetByID), ctx, id)
}

type Mocklogger struct {
	ctrl     *gomock.Controller
	recorder *MockloggerMockRecorder
	isgomock struct{}
}

type MockloggerMockRecorder struct {
	mock *Mocklogger
}

func NewMocklogger(ctrl *gomock.Controller) *Mocklogger {
	mock := &Mocklogger{ctrl: ctrl}
	mock.recorder = &MockloggerMockRecorder{mock}
	return mock
}

func (m *Mocklogger) EXPECT() *MockloggerMockRecorder {
	return m.recorder
}

func (m *Mocklogger) Error(msg string, args ...any) {
	m.ctrl.T.Helper()
	varargs := []any{msg}
	for _, a := range args {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Error", varargs...)
}

func (mr *MockloggerMockRecorder) Error(msg any, args ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{msg}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Error", reflect.TypeOf((*Mocklogger)(nil).Error), varargs...)
}

func (m *Mocklogger) Info(msg string, args ...any) {
	m.ctrl.T.Helper()
	varargs := []any{msg}
	for _, a := range args {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Info", varargs...)
}

func (mr *MockloggerMockRecorder) Info(msg any, args ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{msg}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Info", reflect.TypeOf((*Mocklogger)(nil).Info), varargs...)
}
