package cmd

import (
	"fmt"
	"strconv"
)

func Add(first string, second string, shouldBeInt bool) (result string) {
	num1, err := strconv.ParseFloat(first, 64)
	if err != nil {
		fmt.Println("Error: First value is invalid")
		return
	}
	num2, err := strconv.ParseFloat(second, 64)
	if err != nil {
		fmt.Println("Error: Second value is invalid")
		return
	}
	if shouldBeInt {
		return fmt.Sprintf("%.f", num1+num2)
	}
	return fmt.Sprintf("%.2f", num1+num2)
}

func Subtract(from string, subtract string, shouldBeInt bool) (result string) {
	num1, err := strconv.ParseFloat(from, 64)
	if err != nil {
		fmt.Println("Error: First value is invalid")
		return
	}
	num2, err := strconv.ParseFloat(subtract, 64)
	if err != nil {
		fmt.Println("Error: Second value is invalid")
		return
	}
	if shouldBeInt {
		return fmt.Sprintf("%.f", num1-num2)
	}
	return fmt.Sprintf("%.2f", num1-num2)
}
