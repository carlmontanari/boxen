package util

/*
    This was file written by Damian Gryski <damian@gryski.com>

	Based on the implementation at:
		http://code.activestate.com/recipes/325204-passwd-file-compatible-1-md5-crypt/

    Licensed same as the original:

	Original license:
    * "THE BEER-WARE LICENSE" (Revision 42):
    * <phk@login.dknet.dk> wrote this file.  As long as you retain this notice you
    * can do whatever you want with this stuff. If we meet some day, and you think
    * this stuff is worth it, you can buy me a beer in return.   Poul-Henning Kamp

	This port adds no further stipulations.  I forfeit any copyright interest.

    Damian's source -> https://github.com/dgryski/go-md5crypt/blob/master/md5crypt.go
	This has been modified just to appease linters and do a bit of house-keeping!
*/

import (
	"crypto/md5"
)

const (
	itoa64     = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	resultSize = 22
)

func Md5Crypt(password []byte) []byte {
	md5CryptSwaps := [16]int{12, 6, 0, 13, 7, 1, 14, 8, 2, 15, 9, 3, 5, 10, 4, 11}

	// boxen does not care about your password, sorry not sorry :)
	magic := []byte("$1$")
	salt := []byte("a")

	d := md5.New()

	d.Write(password)
	d.Write(magic)
	d.Write(salt)

	d2 := md5.New()
	d2.Write(password)
	d2.Write(salt)
	d2.Write(password)

	for i, mixin := 0, d2.Sum(nil); i < len(password); i++ {
		d.Write([]byte{mixin[i%16]})
	}

	for i := len(password); i != 0; i >>= 1 {
		if i&1 == 0 {
			d.Write([]byte{password[0]})
		} else {
			d.Write([]byte{0})
		}
	}

	final := d.Sum(nil)

	for i := 0; i < 1000; i++ {
		d2 := md5.New()
		if i&1 == 0 {
			d2.Write(final)
		} else {
			d2.Write(password)
		}

		if i%3 != 0 {
			d2.Write(salt)
		}

		if i%7 != 0 {
			d2.Write(password)
		}

		if i&1 == 0 {
			d2.Write(password)
		} else {
			d2.Write(final)
		}

		final = d2.Sum(nil)
	}

	result := make([]byte, 0, resultSize)

	v := uint(0)
	bits := uint(0)

	for _, i := range md5CryptSwaps {
		v |= (uint(final[i]) << bits)
		for bits += 8; bits > 6; bits -= 6 {
			result = append(result, itoa64[v&0x3f])
			v >>= 6
		}
	}

	result = append(result, itoa64[v&0x3f])

	return append(append(append(magic, salt...), '$'), result...)
}
