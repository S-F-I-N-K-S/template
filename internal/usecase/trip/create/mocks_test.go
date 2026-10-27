package create_test

import (
	context "context"
	reflect "reflect"
	time "time"

	trip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
	uuid "github.com/google/uuid"
	gomock "go.uber.org/mock/gomock"
)

type MocktxManager struct {
	ctrl     *gomock.Controller
	recorder *MocktxManagerMockRecorder
	isgomock struct{}
}

type MocktxManagerMockRecorder struct {
	mock *MocktxManager
}

func NewMocktxManager(ctrl *gomock.Controller) *MocktxManager {
	mock := &MocktxManager{ctrl: ctrl}
	mock.recorder = &MocktxManagerMockRecorder{mock}
	return mock
}

func (m *MocktxManager) EXPECT() *MocktxManagerMockRecorder {
	return m.recorder
}

func (m *MocktxManager) Do(ctx context.Context, fn func(context.Context) error) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Do", ctx, fn)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MocktxManagerMockRecorder) Do(ctx, fn any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Do", reflect.TypeOf((*MocktxManager)(nil).Do), ctx, fn)
}

type MocktripRepository struct {
	ctrl     *gomock.Controller
	recorder *MocktripRepositoryMockRecorder
	isgomock struct{}
}

type MocktripRepositoryMockRecorder struct {
	mock *MocktripRepository
}

func NewMocktripRepository(ctrl *gomock.Controller) *MocktripRepository {
	mock := &MocktripRepository{ctrl: ctrl}
	mock.recorder = &MocktripRepositoryMockRecorder{mock}
	return mock
}

func (m *MocktripRepository) EXPECT() *MocktripRepositoryMockRecorder {
	return m.recorder
}

func (m *MocktripRepository) Save(ctx context.Context, t trip.Trip) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Save", ctx, t)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MocktripRepositoryMockRecorder) Save(ctx, t any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Save", reflect.TypeOf((*MocktripRepository)(nil).Save), ctx, t)
}

type MockhistoryRepository struct {
	ctrl     *gomock.Controller
	recorder *MockhistoryRepositoryMockRecorder
	isgomock struct{}
}

type MockhistoryRepositoryMockRecorder struct {
	mock *MockhistoryRepository
}

func NewMockhistoryRepository(ctrl *gomock.Controller) *MockhistoryRepository {
	mock := &MockhistoryRepository{ctrl: ctrl}
	mock.recorder = &MockhistoryRepositoryMockRecorder{mock}
	return mock
}

func (m *MockhistoryRepository) EXPECT() *MockhistoryRepositoryMockRecorder {
	return m.recorder
}

func (m *MockhistoryRepository) AppendHistory(ctx context.Context, tripID uuid.UUID, from, to, reason string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AppendHistory", ctx, tripID, from, to, reason)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockhistoryRepositoryMockRecorder) AppendHistory(ctx, tripID, from, to, reason any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AppendHistory", reflect.TypeOf((*MockhistoryRepository)(nil).AppendHistory), ctx, tripID, from, to, reason)
}

type Mockclock struct {
	ctrl     *gomock.Controller
	recorder *MockclockMockRecorder
	isgomock struct{}
}

type MockclockMockRecorder struct {
	mock *Mockclock
}

func NewMockclock(ctrl *gomock.Controller) *Mockclock {
	mock := &Mockclock{ctrl: ctrl}
	mock.recorder = &MockclockMockRecorder{mock}
	return mock
}

func (m *Mockclock) EXPECT() *MockclockMockRecorder {
	return m.recorder
}

func (m *Mockclock) Now() time.Time {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Now")
	ret0, _ := ret[0].(time.Time)
	return ret0
}

func (mr *MockclockMockRecorder) Now() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Now", reflect.TypeOf((*Mockclock)(nil).Now))
}

type Mockmetrics struct {
	ctrl     *gomock.Controller
	recorder *MockmetricsMockRecorder
	isgomock struct{}
}

type MockmetricsMockRecorder struct {
	mock *Mockmetrics
}

func NewMockmetrics(ctrl *gomock.Controller) *Mockmetrics {
	mock := &Mockmetrics{ctrl: ctrl}
	mock.recorder = &MockmetricsMockRecorder{mock}
	return mock
}

func (m *Mockmetrics) EXPECT() *MockmetricsMockRecorder {
	return m.recorder
}

func (m *Mockmetrics) TripCreated(ctx context.Context) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "TripCreated", ctx)
}

func (mr *MockmetricsMockRecorder) TripCreated(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TripCreated", reflect.TypeOf((*Mockmetrics)(nil).TripCreated), ctx)
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

type MockidGenerator struct {
	ctrl     *gomock.Controller
	recorder *MockidGeneratorMockRecorder
	isgomock struct{}
}

type MockidGeneratorMockRecorder struct {
	mock *MockidGenerator
}

func NewMockidGenerator(ctrl *gomock.Controller) *MockidGenerator {
	mock := &MockidGenerator{ctrl: ctrl}
	mock.recorder = &MockidGeneratorMockRecorder{mock}
	return mock
}

func (m *MockidGenerator) EXPECT() *MockidGeneratorMockRecorder {
	return m.recorder
}

func (m *MockidGenerator) New() uuid.UUID {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "New")
	ret0, _ := ret[0].(uuid.UUID)
	return ret0
}

func (mr *MockidGeneratorMockRecorder) New() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "New", reflect.TypeOf((*MockidGenerator)(nil).New))
}
