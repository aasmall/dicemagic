// Code generated by "stringer -type=ChannelType"; DO NOT EDIT.

package main

import "strconv"

const _ChannelType_name = "UnknownDMMultiDMStandard"

var _ChannelType_index = [...]uint8{0, 7, 9, 16, 24}

func (i ChannelType) String() string {
	if i >= ChannelType(len(_ChannelType_index)-1) {
		return "ChannelType(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _ChannelType_name[_ChannelType_index[i]:_ChannelType_index[i+1]]
}
