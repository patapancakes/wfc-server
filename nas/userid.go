package nas

var t = [16]byte{
	7, 2, 5, 10,
	11, 0, 13, 15,
	12, 1, 6, 8,
	4, 9, 3, 14,
}

var exc = [8]byte{
	1, 2, 0, 4, 3, 5, 6, 7,
}

func decodeDSUserID(userid uint64) (uint16, uint32, bool, uint8) {
	var b [8]byte

	for i := range b {
		b[i] = byte(userid >> (8 * i))
	}

	for i := range 6 {
		b[i] ^= 0x67
	}

	b[5] &= 0x07
	b[6] = 0
	b[7] = 0

	var v uint64
	for i, x := range b {
		v |= uint64(x) << (8 * i)
	}

	v = (v>>1 | v&1<<42) & (1<<43 - 1)

	for i := range b {
		b[i] = byte(v >> (8 * i))
	}

	tmp := b
	for i, src := range exc {
		b[i] = tmp[src]
	}

	for i := range 5 {
		high := t[b[i]>>4]
		low := t[b[i]&0x0F]
		b[i] = high<<4 | low
	}

	for i := range 6 {
		b[i] ^= 0xD6
	}

	v = 0
	for i, x := range b {
		v |= uint64(x) << (8 * i)
	}

	uid := uint16(v >> 27 & 0xFFFF)
	mac := uint32(v >> 3 & 0xFFFFFF) // last 3 octets of mac
	nintendo := v>>2&1 == 0          // if prefix 00:09:BF
	unk := uint8(v & 0x03)           // always 0?

	return uid, mac, nintendo, unk
}
