package bdb

import (
	"fmt"
	"github.com/go-errors/errors"
	"strconv"
	"strings"
)

type ShortChanId struct {
	BlockHeight uint32
	TxIndex     uint32
	TxPosition  uint16
}

func NewShortChanIdFromString(str string) (ShortChanId, error) {
	shortChanId := ShortChanId{}
	var parts []string

	parts = strings.Split(str, ":")
	if len(parts) != 3 {
		parts = strings.Split(str, "x")
	}

	if len(parts) != 3 {
		return shortChanId, errors.Errorf("Unable to parse short channel id with format 123:45:6 or 123x45x6")
	}

	blockHeight, err := strconv.Atoi(parts[0])
	if err != nil {
		return shortChanId, errors.Errorf("Could not parse block height: %v", err)
	}
	shortChanId.BlockHeight = uint32(blockHeight)

	txIndex, err := strconv.Atoi(parts[1])
	if err != nil {
		return shortChanId, errors.Errorf("Could not parse tx index: %v", err)
	}
	shortChanId.TxIndex = uint32(txIndex)

	txPosition, err := strconv.Atoi(parts[2])
	if err != nil {
		return shortChanId, errors.Errorf("Could not parse tx position: %v", err)
	}
	shortChanId.TxPosition = uint16(txPosition)

	return shortChanId, nil
}

func NewShortChanIdFromInt(chanID uint64) ShortChanId {
	return ShortChanId{
		BlockHeight: uint32(chanID >> 40),
		TxIndex:     uint32(chanID>>16) & 0xFFFFFF,
		TxPosition:  uint16(chanID),
	}
}

func (c ShortChanId) ToUint64() uint64 {
	return (uint64(c.BlockHeight) << 40) | (uint64(c.TxIndex) << 16) | (uint64(c.TxPosition))
}

func (c ShortChanId) String() string {
	return fmt.Sprintf("%d:%d:%d", c.BlockHeight, c.TxIndex, c.TxPosition)
}

func (c ShortChanId) Valid() bool {
	return c.BlockHeight != 0 && c.TxIndex != 0 && c.TxIndex != 0
}
