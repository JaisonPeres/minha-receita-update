package cnpjcanonical

import "errors"

var ErrInvalidCNPJ = errors.New("cnpj inválido: esperado 14 caracteres [0-9A-Z]")
