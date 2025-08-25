package extractors

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/playwright-community/playwright-go"
	"github.com/shredd0r/anki-card-creator/config"
	"github.com/shredd0r/anki-card-creator/models"
)

var errIndexOutOfRange = errors.New("index out of range")

const (
	mainNotesPage                = "https://smarte.greenforest.ua/book/6b83b23209033d606be62c9d237d2b1f01bd82fc/4a814ae3f003aede2422dabd06a31fe8175f30be"
	selector_for_navigation_item = "div[class*=item-list]"
	selector_for_flash_card      = "div.flashcard"
	selector_for_subject         = "div.side-a>div>strong"
	selector_for_pronounce       = "div.side-a>div.inner-block>div>audio" //field 'src'
	selector_for_explain         = "div.side-b>div.inner-block"
)

type notesCardExtractor struct {
	lengthCache *int
	cfg         config.NotesConfig
	logger      *slog.Logger
	browser     playwright.Browser
}

func NewNotesCardExtractor(logger *slog.Logger, cfg *config.Config, browser playwright.Browser) CardExtractor[models.NotesCard] {
	return &notesCardExtractor{
		logger:  logger.With(slog.Any("struct", "notes-card-extractor")),
		cfg:     cfg.NotesConfig,
		browser: browser,
	}
}

func (e *notesCardExtractor) GetLength(ctx context.Context) (*int, error) {
	e.logger.Debug("start method GetLength")
	select {
	case <-ctx.Done():
		{
			e.logger.Debug("context is done, returning from 'GetLength'")
			return nil, ctx.Err()
		}
	default:
		{
			if e.lengthCache != nil {
				return e.lengthCache, nil
			}

			page, err := e.gotoNotesPage(ctx)
			if err != nil {
				return nil, err
			}

			lessonLocators, err := e.getLessonLocators(ctx, page)
			if err != nil {
				return nil, err
			}

			length := len(*lessonLocators)
			e.lengthCache = &length
			return &length, nil
		}
	}
}

func (e *notesCardExtractor) GetLessons(ctx context.Context) (*[]string, error) {
	select {
	case <-ctx.Done():
		{
			e.logger.Debug("context is done, leaving from 'GetLessons'")
			return nil, ctx.Err()
		}
	default:
		{
			page, err := e.gotoNotesPage(ctx)
			if err != nil {
				return nil, err
			}

			lessonLocators, err := e.getLessonLocators(ctx, page)
			if err != nil {
				return nil, err
			}

			lessonLabels := make([]string, len(*lessonLocators))

			for index := range *lessonLocators {
				label, err := (*lessonLocators)[index].InnerText()
				if err != nil {
					e.logger.Error("failed get label from lesson locator", slog.Any("err", err.Error()))
					return nil, err
				}
				lessonLabels[index] = label
			}
			return &lessonLabels, nil
		}
	}
}

func (e *notesCardExtractor) GetCards(ctx context.Context, lessonIndex int) (*[]*models.NotesCard, error) {
	e.logger.Debug(fmt.Sprintf("start get cards from lesson, by index: %d", lessonIndex))
	select {
	case <-ctx.Done():
		{
			e.logger.Debug("context is done, leaving from 'GetCard'")
			return nil, ctx.Err()
		}
	default:
		{
			length, err := e.GetLength(ctx)
			if err != nil {
				return nil, err
			}

			if lessonIndex < *length && lessonIndex > -1 {
				page, err := e.gotoNotesPage(ctx)
				if err != nil {
					return nil, err
				}

				lessonLabels, err := e.GetLessons(ctx)
				if err != nil {
					return nil, err
				}

				lessonLabel := (*lessonLabels)[lessonIndex]
				//This method click on navigation item and wait when page is load
				_, err = e.getWordlistNavigation(ctx, page)
				if err != nil {
					return nil, err
				}
				_, err = e.getNavigationItemWithInnerItems(ctx, page, lessonLabel)
				if err != nil {
					return nil, err
				}

				cardLocators, err := page.Locator(selector_for_flash_card).All()
				if err != nil {
					e.logger.Error("failed get flashcards from page", slog.Any("err", err.Error()))
					return nil, err
				}

				// Create channels for future goroutines, which will get note cards from browser
				chanNoteCards := make(chan *models.NotesCard, len(cardLocators))
				chanError := make(chan error)
				defer func() {
					close(chanNoteCards)
					close(chanError)
				}()

				wg := &sync.WaitGroup{}
				for cardIndex, cardLocator := range cardLocators {
					wg.Add(1)
					go func() {
						defer wg.Done()
						e.logger.Debug("start get card from page", slog.Any("index", cardIndex))
						e.getCardAndPutToChan(ctx, lessonLabel, cardIndex, cardLocator, chanError, chanNoteCards)
						e.logger.Debug("end get card from page", slog.Any("index", cardIndex))
					}()
				}
				wg.Wait()
				return e.getCardsFromChan(ctx, chanError, chanNoteCards)
			}

			return nil, errIndexOutOfRange
		}
	}
}

func (e notesCardExtractor) gotoNotesPage(ctx context.Context) (playwright.Page, error) {
	e.logger.Debug("start method 'gotoNotesPageAndOpenWordList'")

	select {
	case <-ctx.Done():
		{
			e.logger.Debug("context is done, returning from 'gotoNotesPageAndOpenWordlist'")
			return nil, ctx.Err()
		}
	default:
		{
			e.logger.Debug("create new page")
			page, err := e.browser.NewPage()
			if err != nil {
				e.logger.Error("failed create new page", slog.Any("err", err.Error()))
				return nil, err
			}

			e.logger.Debug("go to main page")
			_, err = page.Goto(mainNotesPage)
			if err != nil {
				e.logger.Error("failed go to main Notes page", slog.Any("err", err.Error()))
				return nil, err
			}

			e.logger.Debug("set tokens to local storage")
			_, err = page.Evaluate(fmt.Sprintf(`
					localStorage.setItem("token", "%s");
					localStorage.setItem("token_raw", "%s")`,
				e.cfg.Token,
				e.cfg.TokenRaw))
			if err != nil {
				e.logger.Error("failed set token to localStorage", slog.Any("err", err.Error()))
				return nil, err
			}

			e.logger.Debug("reload web page")
			_, err = page.Reload()
			if err != nil {
				e.logger.Error("failed reload page", slog.Any("err", err.Error()))
				return nil, err
			}

			return page, nil
		}
	}

}

func (e *notesCardExtractor) getWordlistNavigation(ctx context.Context, page playwright.Page) (playwright.Locator, error) {
	return e.getNavigationItemWithInnerItems(ctx, page, "Wordlist")
}

func (e *notesCardExtractor) getNavigationItemWithInnerItems(ctx context.Context, page playwright.Page, navigationItemLabel string) (playwright.Locator, error) {
	navigationItemLocator, err := e.getNavigationItemByLabel(ctx, page, navigationItemLabel)
	if err != nil {
		return nil, err
	}

	e.logger.Debug("click to navigation item")
	err = navigationItemLocator.Click()
	if err != nil {
		e.logger.Error("failed click to navigation item", slog.Any("err", err.Error()))
		return nil, err
	}

	//wait to open inner navigation items
	err = navigationItemLocator.Locator(selector_for_navigation_item).First().WaitFor()
	if err != nil {
		e.logger.Error("failed waiting open inner navigation items", slog.Any("err", err.Error()))
		return nil, err
	}
	return navigationItemLocator, nil
}

func (e *notesCardExtractor) getNavigationItemByLabel(ctx context.Context, page playwright.Page, label string) (playwright.Locator, error) {
	e.logger.Debug("start method 'getNavigationItemByLabel'")
	select {
	case <-ctx.Done():
		{
			e.logger.Debug("context is done, leaving from getNavigationItemByLabel")
			return nil, ctx.Err()
		}
	default:
		{
			e.logger.Debug("get all navigation item locators")

			err := page.Locator(selector_for_navigation_item).First().WaitFor()
			if err != nil {
				e.logger.Error("failed wait for loading navigation items", slog.Any("err", err.Error()))
				return nil, err
			}
			navigationItemLocators, err := page.Locator(selector_for_navigation_item).All()
			if err != nil {
				e.logger.Error("failed get locators with navigation item", slog.Any("err", err.Error()))
				return nil, err
			}

			e.logger.Debug("got locators", slog.Any("length", len(navigationItemLocators)))
			for i, locator := range navigationItemLocators {
				innerText, err := locator.InnerText()
				e.logger.Debug("navigation item locator", slog.Any("index", i), slog.Any("label", innerText))
				if err != nil {
					e.logger.Error("failed get inner text from navigation label", slog.Any("err", err.Error()))
					return nil, err
				}

				if innerText == label {
					return locator, nil
				}
			}

			return nil, errors.New("navigation item not found")
		}
	}
}

func (e *notesCardExtractor) getLessonLocators(ctx context.Context, page playwright.Page) (*[]playwright.Locator, error) {
	select {
	case <-ctx.Done():
		{
			e.logger.Debug("context is done, leaving from 'getLessonLocators'")
			return nil, ctx.Err()
		}
	default:
		{
			wordListLocator, err := e.getWordlistNavigation(ctx, page)
			if err != nil {
				return nil, err
			}

			navigationItemLocators, err := wordListLocator.Locator(selector_for_navigation_item).All()
			if err != nil {
				e.logger.Error("failed get lesson navigation items", slog.Any("err", err.Error()))
				return nil, err
			}

			// Subtract 1, because first navigation item is ads.
			lessonsLocators := make([]playwright.Locator, len(navigationItemLocators)-1)
			copy(lessonsLocators, navigationItemLocators[1:])

			return &lessonsLocators, nil
		}
	}
}

func (e *notesCardExtractor) getCard(ctx context.Context, lessonLabel string, cardLocator playwright.Locator) (*models.NotesCard, error) {
	select {
	case <-ctx.Done():
		{
			e.logger.Debug("context is done, leaving from method 'getCard'")
			return nil, ctx.Err()
		}
	default:
		{
			subjectLocator := cardLocator.Locator(selector_for_subject)
			subject, err := subjectLocator.InnerText()
			if err != nil {
				e.logger.Error("failed get subject from cardLocator", slog.Any("err", err.Error()))
				return nil, err
			}
			explainLocator := cardLocator.Locator(selector_for_explain)
			explain, err := explainLocator.InnerText()
			if err != nil {
				e.logger.Error("failed get inner text from locator", slog.Any("err", err.Error()))
				return nil, err
			}
			subjectType := e.getSubjectType(subject)

			card := &models.NotesCard{
				Subject:     subject,
				SubjectType: subjectType,
				LessonName:  lessonLabel,
				Explain:     explain,
			}
			return card, nil
		}
	}

}

// getCardAndPutToChan is method for putting note card received method 'getCard' to data channel
// Also, this method handle errors, if error happend, this error will be put in error channel
// It is expected, that this method will run in goroutine
func (e *notesCardExtractor) getCardAndPutToChan(ctx context.Context, lessonLabel string, cardIndex int, cardLocator playwright.Locator, chanError chan error, chanNoteCards chan *models.NotesCard) {
	select {
	case <-chanError:
		{
			e.logger.Debug("some goroutine for get card return error, stopping goroutines", slog.Any("cardIndex", cardIndex))
			return
		}
	case <-ctx.Done():
		{
			e.logger.Debug("context is done, leaving from goroutine for get note card")
			chanError <- ctx.Err()
			return
		}
	default:
		{
			e.logger.Debug(fmt.Sprintf("start getting card: %d", cardIndex))
			noteCard, err := e.getCard(ctx, lessonLabel, cardLocator)
			if err != nil {
				e.logger.Error("failed get card from note page", slog.Any("err", err.Error()))
				chanError <- err
				return
			}

			chanNoteCards <- noteCard
		}
	}
}

// getCardsFromChan is method for get all card from data channel and put it to array
// Also, this method handle errors, if error happend, this error will be put in error channel
// It's expected, that this method will run in main goroutine
func (e *notesCardExtractor) getCardsFromChan(ctx context.Context, chanError chan error, chanNoteCards chan *models.NotesCard) (*[]*models.NotesCard, error) {
	var noteCards = make([]*models.NotesCard, len(chanNoteCards))

	select {
	case <-chanError:
		{
			err := <-chanError
			e.logger.Debug("some goroutine for get card return error, stopping 'GetCards'", slog.Any("err", err.Error()))
			return nil, err
		}
	case <-ctx.Done():
		{
			e.logger.Debug("context is done, leaving from 'GetCards'")
			return nil, ctx.Err()
		}
	default:
		{
			for index := range noteCards {
				noteCards[index] = <-chanNoteCards
			}
			return &noteCards, nil
		}
	}
}

func (e *notesCardExtractor) getSubjectType(subject string) models.SubjectType {
	if strings.Contains(subject, " ") {
		return models.SubjectTypePhrase
	}
	return models.SubjectTypeWord
}
