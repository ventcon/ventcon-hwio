package ventilator

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/ansel1/merry/v2"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
	"github.com/ventcon/ventcon-hwio/encoding"
	"github.com/ventcon/ventcon-hwio/serial"
	msgSpec "github.com/ventcon/ventcon-msgspec"
)

func TestNewVentialtorManager(t *testing.T) {
	addr := 1
	vent := Ventilator{
		Address:            addr,
		HasFeatureHumidity: true,
		HasFeatureVOC:      false,
	}

	callBackWasCalled := false

	callBack := func(state *msgSpec.VentilatorState) {
		callBackWasCalled = true
	}
	period := 1 * time.Second

	managerInterface, requstChan := NewVentilatorManager(vent, period, callBack)

	managerStruct, ok := managerInterface.(*ventilatorManager)
	if !ok {
		t.Error("Returned ventilator manager interface is not a ventilator manager struct")
	}

	test.Eq(t, vent, managerStruct.ventilator)
	test.Eq(t, addr, managerStruct.state.Address)
	test.False(t, managerStruct.state.Online)
	test.Eq(t, period, managerStruct.configuredPeriod)
	test.Eq(t, period, managerStruct.currentPeriod)
	test.NotNil(t, managerStruct.ticker)
	test.False(t, managerStruct.hasIntiallyRead)
	test.NotNil(t, managerStruct.requestChannel)
	test.NotNil(t, managerStruct.stop)
	test.NotNil(t, requstChan)

	test.False(t, callBackWasCalled)

	managerStruct.statusUpdateCallback(nil)

	test.True(t, callBackWasCalled)

	req, err := encoding.NewReadRequest(100, 100)
	must.NoError(t, err)

	testReq := serial.Request{
		ResponseChannel: nil,
		Data:            req,
	}
	go func() {
		managerStruct.requestChannel <- testReq
	}()

	test.Eq(t, testReq, <-requstChan)

	close(managerStruct.requestChannel)
	close(managerStruct.stop)
}

func setupTestVentManager(
	t *testing.T,
	vent Ventilator,
	period time.Duration,
	statusUpdateCallback func(state *msgSpec.VentilatorState),
) (*ventilatorManager, <-chan serial.Request) {
	managerInterface, requestChannel := NewVentilatorManager(vent, period, statusUpdateCallback)

	managerStruct, ok := managerInterface.(*ventilatorManager)
	if !ok {
		t.Error("Returned ventilator manager interface is not a ventilator manager struct")
	}

	return managerStruct, requestChannel
}

func setupTestVentManagerWithDefaults(
	t *testing.T,
	statusUpdateCallback func(state *msgSpec.VentilatorState),
) (*ventilatorManager, <-chan serial.Request) {
	return setupTestVentManager(
		t,
		Ventilator{
			Address:            1,
			HasFeatureHumidity: true,
			HasFeatureVOC:      false,
		},
		1*time.Second,
		statusUpdateCallback,
	)
}

func isChannelClose[K any](ch <-chan K) bool {
	select {
	case _, ok := <-ch:
		return !ok
	default:
		return false
	}
}

func TestStartStop(t *testing.T) {
	var mgr *ventilatorManager

	callBack := func(state *msgSpec.VentilatorState) {
		mgr.Stop()
	}

	mgr, reqChan := setupTestVentManager(
		t,
		Ventilator{
			Address:            1,
			HasFeatureHumidity: true,
			HasFeatureVOC:      false,
		},
		5*time.Millisecond,
		callBack,
	)

	go func() {
		req := <-reqChan
		req.ResponseChannel <- serial.Response{
			Err: merry.Errorf("TestError"),
		}
	}()

	mgr.Start()

	// TODO: Replace with using synctest. See #34
	time.Sleep(20 * time.Millisecond) // Wait for ticker to tick, callBack to be called, and manager to be stopped

	test.True(t, isChannelClose(mgr.stop))
	test.True(t, isChannelClose(reqChan))
}

func mapSlice[I, O any](s []I, fn func(I) O) []O {
	var r = make([]O, 0, len(s))

	for _, v := range s {
		r = append(r, fn(v))
	}

	return r
}

func filterSlice[T any](s []T, fn func(T) bool) []T {
	var r = make([]T, 0, len(s))

	for _, v := range s {
		if fn(v) {
			r = append(r, v)
		}
	}

	return r
}

func respondToSingleReq(t *testing.T, req serial.Request) {
	var respFrameType encoding.FrameType
	var value uint16

	if req.Data.FrameType() == encoding.ReadRequest {
		respFrameType = encoding.ReadResponse
		value = 1
	} else {
		respFrameType = encoding.WriteResponse
		value = uint16(req.Data.Value())
	}

	resp, err := encoding.NewResponse(respFrameType, uint16(req.Data.Address()), uint16(req.Data.Function()), value)
	must.NoError(t, err)

	req.ResponseChannel <- serial.Response{
		Response: resp,
	}
}

func listenAndRespond(t *testing.T, reqChan <-chan serial.Request, additionalAction func(req serial.Request)) {
	for {
		req := <-reqChan

		additionalAction(req)

		respondToSingleReq(t, req)
	}
}

func TestReadDataAndUpdateStateReadsAllData(t *testing.T) {
	callBack := func(state *msgSpec.VentilatorState) {}

	mgr, reqChan := setupTestVentManagerWithDefaults(t, callBack)

	var requestedFunctions []int

	go listenAndRespond(t, reqChan, func(req serial.Request) {
		requestedFunctions = append(requestedFunctions, req.Data.Function())
	})

	must.False(t, mgr.hasIntiallyRead)
	must.False(t, mgr.state.Online)

	mgr.ReadDataAndUpdateState(false)

	test.True(t, mgr.hasIntiallyRead)
	test.True(t, mgr.state.Online)

	sortedFunctions := slices.Sorted(slices.Values(requestedFunctions))

	expectedFunctions := mapSlice(
		filterSlice(AllFunctions[:], func(funct Function) bool {
			return funct != VocConcentration
		}),
		func(funct Function) int {
			return int(funct)
		})

	test.Eq(t, expectedFunctions, sortedFunctions)
}

func TestReadDataAndUpdateStateReadsMostDataOnSecondRun(t *testing.T) {
	callBack := func(state *msgSpec.VentilatorState) {}

	mgr, reqChan := setupTestVentManagerWithDefaults(t, callBack)

	firstRunCompleted := false
	var requestedFunctionsSecondRun []int

	go listenAndRespond(t, reqChan, func(req serial.Request) {
		if firstRunCompleted {
			requestedFunctionsSecondRun = append(requestedFunctionsSecondRun, req.Data.Function())
		}
	})

	must.False(t, mgr.hasIntiallyRead)
	must.False(t, mgr.state.Online)

	mgr.ReadDataAndUpdateState(false)

	must.True(t, mgr.hasIntiallyRead)
	must.True(t, mgr.state.Online)

	firstRunCompleted = true

	mgr.ReadDataAndUpdateState(false)

	test.True(t, mgr.hasIntiallyRead)
	test.True(t, mgr.state.Online)

	sortedFunctions := slices.Sorted(slices.Values(requestedFunctionsSecondRun))

	expectedFunctions := mapSlice(
		filterSlice(AllFunctions[:], func(funct Function) bool {
			return funct != VocConcentration && funct != CommanderAndVentilationMode && funct != RequestedIntakeAndExhaustAirlevels
		}),
		func(funct Function) int {
			return int(funct)
		})

	test.Eq(t, expectedFunctions, sortedFunctions)
}

func TestReadDataAndUpdateStateSendsUpdatedStateWithData(t *testing.T) {
	var finalSentState msgSpec.VentilatorState
	callBack := func(state *msgSpec.VentilatorState) {
		finalSentState = *state
	}

	mgr, reqChan := setupTestVentManagerWithDefaults(t, callBack)

	go listenAndRespond(t, reqChan, func(req serial.Request) {})

	must.False(t, mgr.state.Data.RemoteCommander)
	must.False(t, mgr.state.Data.VentilationMode)
	must.Eq(t, 0, mgr.state.Data.RequestedIntakeAirLevel)
	must.Eq(t, 0, mgr.state.Data.RequestedExhaustAirLevel)
	must.Eq(t, 0, mgr.state.Data.ActualIntakeAirLevel)
	must.Eq(t, 0, mgr.state.Data.ActualExhaustAirLevel)
	must.Eq(t, 0, mgr.state.Data.ExternalSwitchPosition)
	must.False(t, mgr.state.Data.FilterInstalled)
	must.False(t, mgr.state.Data.FilterDirty)
	must.False(t, mgr.state.Data.FrostRisk)
	must.Eq(t, 0, mgr.state.Data.ExhaustAirTemp)
	must.Eq(t, 0, mgr.state.Data.RoomAirTemp)
	must.Eq(t, 0, mgr.state.Data.ExternalAirTemp)
	must.Eq(t, 0, mgr.state.Data.InletAirTemp)
	must.Eq(t, 0, mgr.state.Data.ExhaustAirHumidity)
	must.Eq(t, 0, mgr.state.Data.InletAirHumidity)
	must.Eq(t, 0, mgr.state.Data.VocConcentration)
	must.False(t, mgr.state.Online)

	mgr.ReadDataAndUpdateState(false)

	test.True(t, mgr.hasIntiallyRead)
	test.True(t, mgr.state.Online)

	test.True(t, mgr.state.Data.RemoteCommander)
	test.False(t, mgr.state.Data.VentilationMode)
	test.Eq(t, 0, mgr.state.Data.RequestedIntakeAirLevel)
	test.Eq(t, 1, mgr.state.Data.RequestedExhaustAirLevel)
	test.Eq(t, 0, mgr.state.Data.ActualIntakeAirLevel)
	test.Eq(t, 1, mgr.state.Data.ActualExhaustAirLevel)
	test.Eq(t, 1, mgr.state.Data.ExternalSwitchPosition)
	test.True(t, mgr.state.Data.FilterInstalled)
	test.False(t, mgr.state.Data.FilterDirty)
	test.True(t, mgr.state.Data.FrostRisk)
	test.Eq(t, -39.9, mgr.state.Data.ExhaustAirTemp)
	test.Eq(t, -39.9, mgr.state.Data.RoomAirTemp)
	test.Eq(t, -39.9, mgr.state.Data.ExternalAirTemp)
	test.Eq(t, -39.9, mgr.state.Data.InletAirTemp)
	test.Eq(t, 1, mgr.state.Data.ExhaustAirHumidity)
	test.Eq(t, 1, mgr.state.Data.InletAirHumidity)
	test.Eq(t, 0, mgr.state.Data.VocConcentration) // as the ventialtor does not have the feature

	test.True(t, finalSentState.Online)
	test.Eq(t, 1, finalSentState.Address)
	test.Eq(t, mgr.state.Data, finalSentState.Data)
}

func TestReadDataAndUpdateStateErrorDoesNotMarkAsInitiallyRead(t *testing.T) {
	callBack := func(state *msgSpec.VentilatorState) {}

	mgr, reqChan := setupTestVentManagerWithDefaults(t, callBack)

	go func() {
		req := <-reqChan
		req.ResponseChannel <- serial.Response{
			Err: merry.Errorf("TestError"),
		}
	}()

	must.False(t, mgr.hasIntiallyRead)
	must.False(t, mgr.state.Online)

	mgr.ReadDataAndUpdateState(false)

	test.False(t, mgr.hasIntiallyRead)
	test.False(t, mgr.state.Online)
}

func TestReadDataAndUpdateStateNoDataOnSerialErrorMarksAsOffline(t *testing.T) {
	callBack := func(state *msgSpec.VentilatorState) {}

	mgr, reqChan := setupTestVentManagerWithDefaults(t, callBack)

	go func() {
		// For first read request: Sucess
		for range AllFunctions {
			req := <-reqChan
			respondToSingleReq(t, req)
		}
		// Next request: Return no DataOnSerialError
		req2 := <-reqChan
		req2.ResponseChannel <- serial.Response{
			Err: serial.NoDataOnSerialError,
		}
	}()

	must.False(t, mgr.hasIntiallyRead)
	must.False(t, mgr.state.Online)

	mgr.ReadDataAndUpdateState(false)

	test.True(t, mgr.hasIntiallyRead)
	test.True(t, mgr.state.Online)

	mgr.ReadDataAndUpdateState(false)

	test.True(t, mgr.hasIntiallyRead)
	test.False(t, mgr.state.Online)
}

func TestMarkAsOfflineScheduled(t *testing.T) {
	callBack := func(state *msgSpec.VentilatorState) {}

	mgr, _ := setupTestVentManager(
		t,
		Ventilator{
			Address:            1,
			HasFeatureHumidity: true,
			HasFeatureVOC:      false,
		},
		1*time.Minute,
		callBack,
	)

	mgr.state.Online = true

	must.True(t, mgr.state.Online)
	must.Eq(t, 1*time.Minute, mgr.configuredPeriod)
	must.Eq(t, 1*time.Minute, mgr.currentPeriod)

	mgr.MarkAsOffline(true)

	test.False(t, mgr.state.Online)
	must.Eq(t, 1*time.Minute, mgr.configuredPeriod)
	must.Eq(t, 2*time.Minute, mgr.currentPeriod)
	// TODO: Check that ticker was actually reset to the new time. See #51

	mgr.MarkAsOffline(true)

	test.False(t, mgr.state.Online)
	must.Eq(t, 1*time.Minute, mgr.configuredPeriod)
	must.Eq(t, 4*time.Minute, mgr.currentPeriod)

	mgr.MarkAsOffline(true)

	test.False(t, mgr.state.Online)
	must.Eq(t, 1*time.Minute, mgr.configuredPeriod)
	must.Eq(t, 8*time.Minute, mgr.currentPeriod)

	mgr.MarkAsOffline(true)

	test.False(t, mgr.state.Online)
	must.Eq(t, 1*time.Minute, mgr.configuredPeriod)
	must.Eq(t, 8*time.Minute, mgr.currentPeriod)
}

func TestMarkAsOfflineUnscheduled(t *testing.T) {
	callBack := func(state *msgSpec.VentilatorState) {}

	mgr, _ := setupTestVentManagerWithDefaults(t, callBack)

	mgr.state.Online = true

	must.True(t, mgr.state.Online)
	must.Eq(t, 1*time.Second, mgr.configuredPeriod)
	must.Eq(t, 1*time.Second, mgr.currentPeriod)

	mgr.MarkAsOffline(false)

	test.False(t, mgr.state.Online)
	must.Eq(t, 1*time.Second, mgr.configuredPeriod)
	must.Eq(t, 2*time.Second, mgr.currentPeriod)
	// TODO: Check that ticker was actually reset to the new time. See #51

	mgr.MarkAsOffline(false)

	test.False(t, mgr.state.Online)
	must.Eq(t, 1*time.Second, mgr.configuredPeriod)
	must.Eq(t, 2*time.Second, mgr.currentPeriod)
}

func TestMarkAsOnlineAgainDoesNotUpdateAnything(t *testing.T) {
	callBack := func(state *msgSpec.VentilatorState) {}

	mgr, _ := setupTestVentManagerWithDefaults(t, callBack)

	mgr.state.Online = true
	mgr.hasIntiallyRead = true
	mgr.currentPeriod = 42 * time.Second

	must.True(t, mgr.state.Online)
	must.True(t, mgr.hasIntiallyRead)
	must.Eq(t, 42*time.Second, mgr.currentPeriod)

	mgr.MarkAsOnline()

	test.True(t, mgr.state.Online)
	test.True(t, mgr.hasIntiallyRead)
	test.Eq(t, 42*time.Second, mgr.currentPeriod)
}

func TestMarkAsOnline(t *testing.T) {
	callBack := func(state *msgSpec.VentilatorState) {}

	mgr, _ := setupTestVentManagerWithDefaults(t, callBack)

	mgr.state.Online = false
	mgr.hasIntiallyRead = true
	mgr.currentPeriod = 42 * time.Second

	must.False(t, mgr.state.Online)
	must.True(t, mgr.hasIntiallyRead)
	must.Eq(t, 42*time.Second, mgr.currentPeriod)

	mgr.MarkAsOnline()

	test.True(t, mgr.state.Online)
	test.False(t, mgr.hasIntiallyRead)
	test.Eq(t, 1*time.Second, mgr.currentPeriod)
}

func allFunctionsExcept(except ...Function) (ret []Function) {
	for _, s := range AllFunctions {
		if !slices.Contains(except, s) {
			ret = append(ret, s)
		}
	}
	return
}

func TestReadData(t *testing.T) {
	testCases := []struct {
		inputFunctions     []Function
		hasFeatureHumidity bool
		hasFeatureVOC      bool
		requestedFunctions []Function
	}{
		{AllFunctions[:], true, true, AllFunctions[:]},
		{AllFunctions[:], true, false, allFunctionsExcept(VocConcentration)},
		{AllFunctions[:], false, true, allFunctionsExcept(ExhaustAirHumidity, InletAirHumidity)},
		{AllFunctions[:], false, false, allFunctionsExcept(VocConcentration, ExhaustAirHumidity, InletAirHumidity)},
		{[]Function{VocConcentration, ExhaustAirHumidity}, true, true, []Function{VocConcentration, ExhaustAirHumidity}},
		{[]Function{VocConcentration, ExhaustAirHumidity}, true, false, []Function{ExhaustAirHumidity}},
		{[]Function{VocConcentration, ExhaustAirHumidity}, false, false, nil},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf(`ReadData(%v) with hasHumidity=%t and hasVOC=%t requests %v`, tc.requestedFunctions, tc.hasFeatureHumidity, tc.hasFeatureVOC, tc.requestedFunctions), func(t *testing.T) {
			callBack := func(state *msgSpec.VentilatorState) {}
			mgr, reqChan := setupTestVentManager(
				t,
				Ventilator{
					Address:            1,
					HasFeatureHumidity: tc.hasFeatureHumidity,
					HasFeatureVOC:      tc.hasFeatureVOC,
				},
				1*time.Second,
				callBack,
			)

			var requestedFunctions []int

			go listenAndRespond(t, reqChan, func(req serial.Request) {
				requestedFunctions = append(requestedFunctions, req.Data.Function())
			})

			err := mgr.ReadData(tc.inputFunctions)
			test.NoError(t, err)

			test.Eq(t, tc.requestedFunctions, requestedFunctions)
		})
	}
}

func TestSendWriteRequestAndUpdateStateReturnsIfOffline(t *testing.T) {
	callBack := func(state *msgSpec.VentilatorState) {}

	mgr, reqChan := setupTestVentManagerWithDefaults(t, callBack)

	var requestedFunctions []int

	go listenAndRespond(t, reqChan, func(req serial.Request) {
		requestedFunctions = append(requestedFunctions, req.Data.Function())
	})

	must.False(t, mgr.state.Online)

	err := mgr.SendWriteRequestAndUpdateState(CommanderAndVentilationMode, 0)

	test.ErrorIs(t, err, VentilatorIsOffline)
	test.Nil(t, requestedFunctions)

}

func TestSendWriteRequestAndUpdateState(t *testing.T) {
	var receivedCallbacks []msgSpec.VentilatorState
	callBack := func(state *msgSpec.VentilatorState) {
		receivedCallbacks = append(receivedCallbacks, *state)
	}

	mgr, reqChan := setupTestVentManagerWithDefaults(t, callBack)

	var requests []encoding.Frame

	go listenAndRespond(t, reqChan, func(req serial.Request) {
		requests = append(requests, req.Data)
	})

	mgr.state.Online = true

	must.True(t, mgr.state.Online)
	must.Eq(t, false, mgr.state.Data.RemoteCommander)
	must.Eq(t, false, mgr.state.Data.VentilationMode)

	expectedWriteRequest, err := encoding.NewWriteRequest(1, 6, 3)
	must.NoError(t, err)

	err = mgr.SendWriteRequestAndUpdateState(CommanderAndVentilationMode, 3)

	test.NoError(t, err)
	test.Eq(t, []encoding.Frame{
		expectedWriteRequest,
	}, requests)

	test.Eq(t, true, mgr.state.Data.RemoteCommander)
	test.Eq(t, true, mgr.state.Data.VentilationMode)

	test.Eq(t, []msgSpec.VentilatorState{
		{
			Address: 1,
			Online:  true,
			Data: msgSpec.VentilatorData{
				RemoteCommander:          true,
				VentilationMode:          true,
				RequestedIntakeAirLevel:  0,
				RequestedExhaustAirLevel: 0,
				ActualIntakeAirLevel:     0,
				ActualExhaustAirLevel:    0,
				ExternalSwitchPosition:   0,
				FilterInstalled:          false,
				FilterDirty:              false,
				FrostRisk:                false,
				ExhaustAirTemp:           0.0,
				RoomAirTemp:              0.0,
				ExternalAirTemp:          0.0,
				InletAirTemp:             0.0,
				ExhaustAirHumidity:       0.0,
				InletAirHumidity:         0,
				VocConcentration:         0,
			},
		},
	}, receivedCallbacks)

}

func TestSendWriteRequestAndUpdateStateSomeError(t *testing.T) {
	var receivedCallbacks []msgSpec.VentilatorState
	callBack := func(state *msgSpec.VentilatorState) {
		receivedCallbacks = append(receivedCallbacks, *state)
	}

	mgr, reqChan := setupTestVentManagerWithDefaults(t, callBack)

	var requests []encoding.Frame

	go listenAndRespond(t, reqChan, func(req serial.Request) {
		requests = append(requests, req.Data)
	})

	mgr.state.Online = true

	must.True(t, mgr.state.Online)
	must.Eq(t, false, mgr.state.Data.RemoteCommander)
	must.Eq(t, false, mgr.state.Data.VentilationMode)

	err := mgr.SendWriteRequestAndUpdateState(CommanderAndVentilationMode, -3)

	test.Error(t, err)
	test.Eq(t, nil, requests)

	test.Eq(t, false, mgr.state.Data.RemoteCommander)
	test.Eq(t, false, mgr.state.Data.VentilationMode)

	test.Eq(t, nil, receivedCallbacks)
}

func TestSendWriteRequestAndUpdateStateNoDataError(t *testing.T) {
	var receivedCallbacks []msgSpec.VentilatorState
	callBack := func(state *msgSpec.VentilatorState) {
		receivedCallbacks = append(receivedCallbacks, *state)
	}

	mgr, reqChan := setupTestVentManagerWithDefaults(t, callBack)

	go func() {
		req := <-reqChan
		req.ResponseChannel <- serial.Response{
			Err: serial.NoDataOnSerialError,
		}
	}()

	mgr.state.Online = true

	must.True(t, mgr.state.Online)
	must.Eq(t, false, mgr.state.Data.RemoteCommander)
	must.Eq(t, false, mgr.state.Data.VentilationMode)

	err := mgr.SendWriteRequestAndUpdateState(CommanderAndVentilationMode, 3)

	test.ErrorIs(t, err, serial.NoDataOnSerialError)

	test.Eq(t, false, mgr.state.Data.RemoteCommander)
	test.Eq(t, false, mgr.state.Data.VentilationMode)

	test.Eq(t, nil, receivedCallbacks)
	test.False(t, mgr.state.Online)
}

func TestRunCommand(t *testing.T) {
	testCases := []struct {
		command          msgSpec.Command
		expectedFunction Function
		expectedValue    int
	}{
		{&msgSpec.PollVentilatorNowCommand{}, CommanderAndVentilationMode, 0},
		{&msgSpec.SetRemoteCommanderCommand{RemoteCommander: true}, CommanderAndVentilationMode, 1},
		{&msgSpec.SetVentilationModeCommand{VentilationMode: true}, CommanderAndVentilationMode, 2},
		{&msgSpec.SetIntakeAirLevelCommand{IntakeAirLevel: 1}, RequestedIntakeAndExhaustAirlevels, 16},
		{&msgSpec.SetExhaustAirLevelCommand{ExhaustAirLevel: 1}, RequestedIntakeAndExhaustAirlevels, 1},
		{&msgSpec.SetBothAirLevelCommand{ExhaustAirLevel: 1, IntakeAirLevel: 1}, RequestedIntakeAndExhaustAirlevels, 17},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf(`RunCommand(%T%v) requests function %d with value %d`, tc.command, tc.command, tc.expectedFunction, tc.expectedValue), func(t *testing.T) {
			var receivedCallbacks []msgSpec.VentilatorState
			callBack := func(state *msgSpec.VentilatorState) {
				receivedCallbacks = append(receivedCallbacks, *state)
			}

			mgr, reqChan := setupTestVentManagerWithDefaults(t, callBack)

			var requests []encoding.Frame

			go listenAndRespond(t, reqChan, func(req serial.Request) {
				requests = append(requests, req.Data)
			})

			mgr.state.Online = true

			must.True(t, mgr.state.Online)
			must.Eq(t, false, mgr.state.Data.RemoteCommander)
			must.Eq(t, false, mgr.state.Data.VentilationMode)

			expectedWriteRequest, err := encoding.NewWriteRequest(1, tc.expectedFunction, tc.expectedValue)
			must.NoError(t, err)

			err = mgr.RunCommand(tc.command)

			test.NoError(t, err)
			_, isPollNow := tc.command.(*msgSpec.PollVentilatorNowCommand)
			if isPollNow {
				test.Greater(t, 0, len(requests))
				expectedReadReques, err := encoding.NewReadRequest(1, tc.expectedFunction)
				must.NoError(t, err)
				test.Eq(t, expectedReadReques, requests[0])
			} else {
				test.Eq(t, []encoding.Frame{
					expectedWriteRequest,
				}, requests)
			}
		})
	}
}
