package uuid

import (
	"strings"

	"github.com/google/uuid"
)

func GetUUID() string {
	return uuid.New().String()
}

func GetPureUUID() string {
	uuidStr := GetUUID()
	return strings.ReplaceAll(uuidStr, "-", "")
}

func GetUUIDInt() uint32 {
	return uuid.New().ID()
}
