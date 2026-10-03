package internal

type partService struct {
	repository *partrepository
}

func NewPartService(repository *partrepository) *partService {
	return &partService{
		repository: repository,
	}
}

func (s *partService) GetAllParts() []Part {
	return s.repository.GetAll()
}

func (s *partService) CreatePart(name string, partType string, weight float64, quantity int) Part {
	part := Part{
		Name:     name,
		Type:     partType,
		Weight:   weight,
		Quantity: quantity,
	}
	part = s.repository.Create(part)
	return part
}

func (s *partService) DeletePart(id int64) error {
	s.repository.Delete(id)
	return nil
}
