package repository

import "github.com/jonathanhu237/rota/backend/internal/model"

// Publication and shift-change flows still surface the legacy shift not found
// sentinel until the assignment/publication refactor lands.
var ErrTemplateShiftNotFound = model.ErrTemplateShiftNotFound
