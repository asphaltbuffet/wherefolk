package rolo

import (
	"fmt"
	"strings"
)

type Person struct {
	Name    string `json:"name"`
	Aka     string `json:"aka"`
	Surname string `json:"surname"`
	Given   string `json:"birth_name"`
	Birth   string `json:"birth"`
	Death   string `json:"death"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
}

func (p Person) Details() []string {

	return []string{
		fmt.Sprintf("%s:", strings.Split(p.Name, " ")[0]),
		p.Birth,
		p.Death,
		p.Phone,
		p.Email,
	}
}

func (p Person) getName() string {
	name := []string{p.Name}

	if p.Given != "" {
		name = append(name, fmt.Sprintf("(%s)", p.Given))
	}
	name = append(name, p.Surname)

	return strings.Join(name, " ")
}
