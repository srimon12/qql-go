package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/output"
)

func toLowerStr(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}

func float32PtrFromFloat64(value *float64) *float32 {
	if value == nil {
		return nil
	}
	converted := float32(*value)
	return &converted
}

func uint32PtrFromInt(value *int) *uint32 {
	if value == nil {
		return nil
	}
	converted := uint32(*value)
	return &converted
}

func newPointID(value any) *qdrant.PointId {
	switch id := value.(type) {
	case int:
		return qdrant.NewIDNum(uint64(id))
	case uint64:
		return qdrant.NewIDNum(id)
	case string:
		return qdrant.NewIDUUID(id)
	default:
		return qdrant.NewIDUUID(fmt.Sprintf("%v", value))
	}
}

func turboBitsEnum(value float64) *qdrant.TurboQuantBitSize {
	switch value {
	case 1.0:
		return qdrant.TurboQuantBitSize_Bits1.Enum()
	case 1.5:
		return qdrant.TurboQuantBitSize_Bits1_5.Enum()
	case 2.0:
		return qdrant.TurboQuantBitSize_Bits2.Enum()
	case 4.0:
		return qdrant.TurboQuantBitSize_Bits4.Enum()
	default:
		return nil
	}
}

func pointIDString(id *qdrant.PointId) string {
	if id == nil {
		return ""
	}
	switch value := pointIDValue(id).(type) {
	case string:
		return value
	case uint64:
		return strconv.FormatUint(value, 10)
	}
	return fmt.Sprintf("%v", id)
}

func pointIDValue(id *qdrant.PointId) any {
	if id == nil {
		return ""
	}
	if uuid := id.GetUuid(); uuid != "" {
		return uuid
	}
	if num, ok := id.GetPointIdOptions().(*qdrant.PointId_Num); ok {
		return num.Num
	}
	return fmt.Sprintf("%v", id)
}

func groupIDString(id *qdrant.GroupId) string {
	if id == nil {
		return ""
	}
	if value := id.GetStringValue(); value != "" {
		return value
	}
	if value := id.GetUnsignedValue(); value != 0 {
		return strconv.FormatUint(value, 10)
	}
	if value := id.GetIntegerValue(); value != 0 {
		return strconv.FormatInt(value, 10)
	}
	return ""
}

func parseUint64(s string) (uint64, error) {
	var n uint64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid number")
		}
		n = n*10 + uint64(c-'0')
	}
	return n, nil
}

func writeJSON(out *output.Outputter, value any, quiet bool) error {
	if err := out.PrintJSON(value, !quiet); err != nil {
		return fmt.Errorf("failed to write JSON: %w", err)
	}
	return nil
}

func commandError(out *output.Outputter, mode commandOutputMode, command, query string, err error) error {
	if mode.json {
		if jsonErr := out.PrintJSON(&ErrorResponse{
			OK:        false,
			Command:   command,
			Query:     query,
			Error:     err.Error(),
			ErrorType: "runtime_error",
		}, false); jsonErr != nil {
			return fmt.Errorf("failed to write JSON: %w", jsonErr)
		}
		return NewExitError(err, 1, true)
	}

	out.PrintError(err.Error())
	return NewExitError(err, 1, true)
}

func currentVersion() string {
	return strings.TrimSpace(Version)
}

func displayVersion() string {
	version := currentVersion()
	if version == "" {
		return "dev"
	}
	return version
}

func versionMessage() string {
	return fmt.Sprintf("qql-go %s", displayVersion())
}
