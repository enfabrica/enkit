package knetwork

import "net"

func IPsToString(i []net.IP) []string{
	var toReturn []string
	for _, v := range i {
		toReturn = append(toReturn, v.String())
	}
	return toReturn
}


func IPsToBytes(i []net.IP) [][]byte{
	var toReturn [][]byte
	for _, v := range i {
		toReturn = append(toReturn, []byte(v.String()))
	}
	return toReturn
}
