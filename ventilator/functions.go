package ventilator

type Function = int

const (
	CommanderAndVentilationMode        Function = 06
	RequestedIntakeAndExhaustAirlevels Function = 13
	ActualIntakeAndExhaustAirlevels    Function = 23
	ExternalSwitchPosition             Function = 30
	FilterStateAndFrostRisk            Function = 34
	ExhaustAirTemp                     Function = 40
	RoomAirTemp                        Function = 41
	ExternalAirTemp                    Function = 42
	InletAirTemp                       Function = 43
	ExhaustAirHumidity                 Function = 44
	InletAirHumidity                   Function = 45
	VocConcentration                   Function = 48
)

var AllFunctions = [...]Function{
	CommanderAndVentilationMode,
	RequestedIntakeAndExhaustAirlevels,
	ActualIntakeAndExhaustAirlevels,
	ExternalSwitchPosition,
	FilterStateAndFrostRisk,
	ExhaustAirTemp,
	RoomAirTemp,
	ExternalAirTemp,
	InletAirTemp,
	ExhaustAirHumidity,
	InletAirHumidity,
	VocConcentration,
}
