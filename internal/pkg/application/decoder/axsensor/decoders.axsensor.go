package axsensor

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"time"

	"github.com/diwise/iot-agent/internal/pkg/application"
	"github.com/diwise/iot-agent/pkg/lwm2m"
)

type AxsensorPayload struct {
	FillingPercentage *float64 `json:"fillingPercentage"`
	FillingLevel      *int64   `json:"fillinglevel,omitempty"`
	Pressure          *float64 `json:"pressure,omitempty"`
	Temperature       *float64 `json:"temperature,omitempty"`
	RelativeHumidity  *float64 `json:"relativeHumidity,omitempty"`
	Vbat              *float64 `json:"vbat,omitempty"`
}

func Decoder(ctx context.Context, deviceID string, e application.SensorEvent) ([]lwm2m.Lwm2mObject, error) {

	if e.FPort != 2 {
		return nil, errors.New("invalid fPort")
	}

	p, err := decode(e.Data)
	if err != nil {
		return nil, err
	}

	return convertToLwm2mObjects(deviceID, p, e.Timestamp), nil
}

func decode(b []byte) (AxsensorPayload, error) {
	p := AxsensorPayload{}
	slen := len(b)
	idx := 0
	blen := slen / 2

	for idx < blen {
		switch b[idx] {
		case 0x80:
			byteValue := int16(binary.LittleEndian.Uint16(b[idx+1 : idx+3]))
			level := float64(1400.0 - byteValue/10.0) //472
			perc := level * 100 / 1400
			levelPercentage := roundFloat(perc, 5)

			p.FillingPercentage = &levelPercentage

			levelCM := int64((level + 5) / 10) // convert from mm to cm
			p.FillingLevel = &levelCM
		case 0xA1:
			pressure := (float64(b[idx+1]) + float64(b[idx+2])*256) * 100 //Pa
			p.Pressure = &pressure
		case 0xA2:
			temperature := (binary.LittleEndian.Uint16(b[idx+1 : idx+3])) //C
			temperatureLevel := float64(temperature) / 10
			p.Temperature = &temperatureLevel
		case 0xA3:
			humidity := (float64(b[idx+1]) + float64(b[idx+2])*256) / 1024 * 100 //rh%
			p.RelativeHumidity = &humidity
		case 0xA4:
			vbat := binary.LittleEndian.Uint16(b[idx+1 : idx+3]) //mV
			batteryLevel := float64(vbat)
			p.Vbat = &batteryLevel
		}
		switch {
		case b[idx] < 0x40:
			idx++
		case b[idx] < 0x80:
			idx += 2
		case b[idx] < 0xC0:
			idx += 3
		default:
			idx += 5
		}
	}
	return p, nil

}

func convertToLwm2mObjects(deviceID string, p AxsensorPayload, ts time.Time) []lwm2m.Lwm2mObject {
	objects := []lwm2m.Lwm2mObject{}

	if p.FillingPercentage != nil {
		fl := lwm2m.NewFillingLevel(deviceID, float64(*p.FillingPercentage), ts)
		fl.ActualFillingLevel = p.FillingLevel

		objects = append(objects, fl)
	}

	if p.Pressure != nil {
		objects = append(objects, lwm2m.NewPressure(deviceID, float64(*p.Pressure), ts))
	}

	if p.RelativeHumidity != nil {
		objects = append(objects, lwm2m.NewHumidity(deviceID, float64(*p.RelativeHumidity), ts))
	}

	if p.Temperature != nil {
		objects = append(objects, lwm2m.NewTemperature(deviceID, float64(*p.Temperature), ts))
	}

	if p.Vbat != nil {
		d := lwm2m.NewDevice(deviceID, ts)
		bat := int(*p.Vbat)
		d.PowerSourceVoltage = &bat
		objects = append(objects, d)
	}

	return objects
}

func roundFloat(val float64, precision uint) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}
