package payout

import (
	"fmt"
	"strconv"
)

const (
	// subgame constants
	RED_BLACK           = 0
	ODD_EVEN            = 1
	LOW_UPPER_HALF      = 2
	COLUMNS             = 3
	LOWER_MID_UPPER_3RD = 4
	EXACT               = 5
)

func getRandNum(hash string) (int, error) {
	var num int64
	var err error
	rand := -1

	if len(hash) > 0 {
		num, err = strconv.ParseInt(hash[0:7], 16, 64)
		if err != nil {
			return rand, fmt.Errorf("hash '%s' is malformed - %s", hash, err.Error())
		}
	} else {
		return -1, fmt.Errorf("hash is missing - %s", err.Error())
	}
	
	rand = int(num % 37)
	return rand, nil
}

func winner(subgame, chipspot, randNum int) bool {

	switch subgame {
	case RED_BLACK:
		if randNum == 0 {
			return false
		}
		// 0 == red
		// 1 == black
		if chipspot == 0 {
			if randNum == 1  || randNum == 3  || randNum == 5  || randNum == 7  || randNum == 9 ||
			   randNum == 12 || randNum == 14 || randNum == 16 || randNum == 18 ||
			   randNum == 19 || randNum == 21 || randNum == 23 || randNum == 25 || randNum == 27 ||
			   randNum == 30 || randNum == 32 || randNum == 34 || randNum == 36 {
				return true
			} else {
				return false
			}
		} else if chipspot == 1 {
			if randNum == 2  || randNum == 4  || randNum == 6  || randNum == 8  ||
			   randNum == 10 || randNum == 11 || randNum == 13 || randNum == 15 || randNum == 17 ||
			   randNum == 20 || randNum == 22 || randNum == 24 || randNum == 26 ||
			   randNum == 28 || randNum == 29 || randNum == 31 || randNum == 33 || randNum == 35 {
				return true
			} else {
				return false
			}
		}
	case ODD_EVEN:
		if randNum == 0 {
			return false
		}
		// 0 == even
		// 1 == odd
		if chipspot % 2 == randNum % 2 {
			return true
		} else {
			return false
		}
	case LOW_UPPER_HALF:
		if randNum == 0 {
			return false
		}
		// 10 (1-18)
		// 28 (19-36)
		if chipspot == 10 {
			if randNum >= 1 && randNum <= 18 {
				return true
			} else {
				return false
			}
		} else if chipspot == 28 {
			if randNum >= 19 && randNum <= 36 {
				return true
			} else {
				return false
			}
		}
	case COLUMNS:
		if randNum == 0 {
			return false
		}
		// 1 (1st column)
		// 2 (2nd column)
		// 3 (3rd column)
		if chipspot == 1 {
			if randNum == 3  || randNum == 6  || randNum == 9  || randNum == 12 ||
			   randNum == 15 || randNum == 18 || randNum == 21 || randNum == 24 || 
			   randNum == 27 || randNum == 30 || randNum == 33 || randNum == 36 {
				return true
			} else {
				return false
			}
		} else if chipspot == 2 {
			if randNum == 2  || randNum == 5  || randNum == 8  || randNum == 11 ||
			   randNum == 14 || randNum == 17 || randNum == 20 || randNum == 23 || 
			   randNum == 26 || randNum == 29 || randNum == 32 || randNum == 35 {
				return true
			} else {
				return false
			}
		} else if chipspot == 3 {
			if randNum == 1  || randNum == 4  || randNum == 7  || randNum == 10 ||
			   randNum == 13 || randNum == 16 || randNum == 19 || randNum == 22 || 
			   randNum == 25 || randNum == 28 || randNum == 31 || randNum == 34 {
				return true
			} else {
				return false
			}
		}
	case LOWER_MID_UPPER_3RD:
		if randNum == 0 {
			return false
		}
		// 6 (1-12)
		// 18 (13-24)
		// 30 (25-36)
		if chipspot == 6 {
			if randNum >= 1 && randNum <= 12 {
				return true
			} else {
				return false
			}
		} else if chipspot == 18 {
			if randNum >= 13 && randNum <= 24 {
				return true
			} else {
				return false
			}
		} else if chipspot == 30 {
			if randNum >= 25 && randNum <= 36 {
				return true
			} else {
				return false
			}
		}
	case EXACT:
		if chipspot == randNum {
			return true
		} else {
			return false
		}
	default:
		return false
	}
	
	return false
}