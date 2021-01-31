package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"github.com/juju/errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kirsrus/termopad/server2/service/web/graph/generated"
	"github.com/kirsrus/termopad/server2/service/web/graph/model"
)

func (r *queryResolver) Config(ctx context.Context) (*model.Config, error) {
	_ = ctx
	config := model.Config{
		TermopadsOnPage: int(r.termopadsOnPage),
		MaxTemperature:  r.maxTemperature,
		MinTemperature:  r.minTemperature,
	}
	return &config, nil
}

func (r *queryResolver) Termopads(ctx context.Context) ([]*model.Termopad, error) {
	_ = ctx
	result := make([]*model.Termopad, 0)
	for _, v := range r.termopads {
		result = append(result, &model.Termopad{
			ID:             strconv.Itoa(int(v.ID)),
			CrateAt:        time.Now().Format("2006.01.02 15:04:05"),
			Address:        v.URL,
			Name:           v.Name,
			Description:    &v.Description,
			MaxTemperature: r.maxTemperature,
			MinTemperature: r.minTemperature,
		})
	}
	return result, nil
}

func (r *queryResolver) Termopad(ctx context.Context, id string) (*model.Termopad, error) {
	_ = ctx
	termID, err := strconv.Atoi(id)
	if err != nil {
		return nil, errors.Errorf("некорректный идентификатор термапада ID:%s: %v", id, err)
	}
	for _, term := range r.termopads {
		if term.ID == uint(termID) {
			return &model.Termopad{
				ID:             strings.TrimSpace(id),
				SudosID:        int(term.SudosID),
				CrateAt:        "",
				Address:        term.URL,
				Name:           term.Name,
				Description:    &term.Description,
				MaxTemperature: r.maxTemperature,
				MinTemperature: r.minTemperature,
			}, nil
		}
	}
	return nil, errors.Errorf("термопада с ID:%s не обнаружено", id)
}

func (r *queryResolver) LastPersons(ctx context.Context) ([]*model.LastPerson, error) {
	_ = ctx
	// Список последних персон, зарегистрировавашихся на термопаде, чтобы показывать
	// их при первой загрузке страницы
	lastPerson := make([]*model.LastPerson, 0)
	for _, termInfo := range r.termopads {
		personDb, err := r.db.LastPerson(termInfo.ID)
		if err != nil {
			if r.db.IsNotFound(err) {
				continue
			}
			return nil, errors.Trace(err)
		}

		lastPerson = append(lastPerson, &model.LastPerson{
			ID:             strconv.Itoa(int(termInfo.ID)),
			UpdateAt:       personDb.CreatedAt.Format("2006.01.02 15:04:05"),
			Image:          personDb.Termperature.ImagePath,
			Wigand:         strconv.Itoa(int(personDb.Termperature.Wigand.ID)),
			WigandFasality: strconv.Itoa(int(personDb.Termperature.Wigand.Fasality())),
			WigandNumber:   strconv.Itoa(int(personDb.Termperature.Wigand.Number())),
			Temperature:    personDb.Termperature.Temperature,
			NameFirst:      &personDb.Person.Name,
			NameMiddle:     &personDb.Person.MiddleName,
			NameLast:       &personDb.Person.Family,
			Organization:   &personDb.Person.Organization,
			Departament:    &personDb.Person.Department,
			Postion:        &personDb.Person.Position,
		})
	}
	return lastPerson, nil
}

func (r *queryResolver) PersonLog(ctx context.Context, id string, days int, offsetDays int, compact bool) ([]*model.TemperatureLogMetric, error) {
	_ = ctx
	wigandID, err := strconv.Atoi(id)
	if err != nil {
		return nil, errors.Errorf("некорректный идентификатор вигадна: %s", id)
	}
	rows, err := r.db.PersonLog(uint(wigandID), uint(days), uint(offsetDays), compact)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// Формирование результата
	result := make([]*model.TemperatureLogMetric, 0)
	for _, v := range rows {
		result = append(result, &model.TemperatureLogMetric{
			Date:           v.Date.Format("2006.01.02 15:04:05"),
			Temperature:    v.Temperature,
			TemperatureMax: v.TemperatureMax,
			TemperatureMin: v.TemperatureMin,
			Image:          v.Image,
			PCreateAt:      v.Person.CreateAt.Format("2006.01.02 15:04:05"),
			PUpdateAt:      v.Person.UpdateAt.Format("2006.01.02 15:04:05"),
			PWigand:        int(v.Person.Wigand.ID),
			PFirstName:     v.Person.Name,
			PMiddleName:    v.Person.MiddleName,
			PLastName:      v.Person.Family,
			POrganization:  v.Person.Organization,
			PDepartament:   v.Person.Department,
			PPosition:      v.Person.Position,
			TID:            int(v.Termopad.ID),
			TURL:           v.Termopad.URL,
			TSudosID:       int(v.Termopad.SudosID),
			TName:          v.Termopad.Name,
			TSerial:        strconv.Itoa(int(v.Termopad.SerialNumber)),
			TDescription:   v.Termopad.Description,
		})
	}
	return result, nil
}

func (r *queryResolver) TermopadLog(ctx context.Context, id string, days int, offsetDays int, compact bool) ([]*model.TemperatureLogMetric, error) {
	_ = ctx
	wigandID, err := strconv.Atoi(id)
	if err != nil {
		return nil, errors.Errorf("некорректный идентификатор термопада: %s", id)
	}
	rows, err := r.db.TermopadLog(uint(wigandID), uint(days), uint(offsetDays), compact)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// Формирование результата
	result := make([]*model.TemperatureLogMetric, 0)
	for _, v := range rows {
		result = append(result, &model.TemperatureLogMetric{
			Date:           v.Date.Format("2006.01.02 15:04:05"),
			Temperature:    v.Temperature,
			TemperatureMax: v.TemperatureMax,
			TemperatureMin: v.TemperatureMin,
			Image:          v.Image,
			PCreateAt:      v.Person.CreateAt.Format("2006.01.02 15:04:05"),
			PUpdateAt:      v.Person.UpdateAt.Format("2006.01.02 15:04:05"),
			PWigand:        int(v.Person.Wigand.ID),
			PFirstName:     v.Person.Name,
			PMiddleName:    v.Person.MiddleName,
			PLastName:      v.Person.Family,
			POrganization:  v.Person.Organization,
			PDepartament:   v.Person.Department,
			PPosition:      v.Person.Position,
			TID:            int(v.Termopad.ID),
			TURL:           v.Termopad.URL,
			TSudosID:       int(v.Termopad.SudosID),
			TName:          v.Termopad.Name,
			TSerial:        strconv.Itoa(int(v.Termopad.SerialNumber)),
			TDescription:   v.Termopad.Description,
		})
	}
	return result, nil
}

func (r *subscriptionResolver) TemperatureChanged(ctx context.Context) (<-chan *model.Temperature, error) {
	// Подписка нового кликнта
	id := uuid.New().String()               // Новый идентификатор канала в пуле каналов
	ch := make(chan *model.Temperature, 10) // Новый канал для передачи данных подписавшемуся
	r.temperatureSubscribePool.Store(id, ch)
	r.log.Debugf("добавлен канал %s в подписку TemperatureChanged", id)
	go func() {
		<-ctx.Done()
		r.temperatureSubscribePool.Delete(id)
		r.log.Debugf("удалён канал %s из подписки TemperatureChanged", id)
	}()

	return ch, nil
}

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

// Subscription returns generated.SubscriptionResolver implementation.
func (r *Resolver) Subscription() generated.SubscriptionResolver { return &subscriptionResolver{r} }

type queryResolver struct{ *Resolver }
type subscriptionResolver struct{ *Resolver }
