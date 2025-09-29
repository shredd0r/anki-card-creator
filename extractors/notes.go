package extractors

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/playwright-community/playwright-go"
	"github.com/shredd0r/anki-card-creator/config"
	"github.com/shredd0r/anki-card-creator/downloader"
)

const (
	main_notes_page                    = "https://smarte.greenforest.ua/book/6b83b23209033d606be62c9d237d2b1f01bd82fc/4a814ae3f003aede2422dabd06a31fe8175f30be"
	selector_for_navigation_item_notes = "div[class*=item-list]"
	selector_for_flash_card_notes      = "div.flashcard"
	selector_for_subject_notes         = "div.side-a>div>strong"
)

var errIndexOutOfRange = errors.New("index out of range")

type NotesExtractor interface {
	GetLessonsLength(ctx context.Context) (*int, error)
	GetLessons(ctx context.Context) (*[]string, error)
	GetSubjectsByLesson(ctx context.Context, indexOfLesson int) (*[]string, error)
}

type implNotesExtractor struct {
	lessonsLengthCache *int
	cfg                config.NotesConfig
	logger             *slog.Logger
	browser            playwright.Browser
	fileDownloader     downloader.FileDownloader
}

func NewNotesCardExtractor(logger *slog.Logger, cfg *config.Config, browser playwright.Browser, fileDownloader downloader.FileDownloader) NotesExtractor {
	return &implNotesExtractor{
		logger:         logger.With(slog.Any("struct", "notes-card-extractor")),
		cfg:            cfg.NotesConfig,
		browser:        browser,
		fileDownloader: fileDownloader,
	}
}

func (e *implNotesExtractor) GetLessonsLength(ctx context.Context) (*int, error) {
	e.logger.Debug("start method GetLength")
	select {
	case <-ctx.Done():
		{
			e.logger.Debug("context is done, returning from 'GetLength'")
			return nil, ctx.Err()
		}
	default:
		{
			if e.lessonsLengthCache != nil {
				return e.lessonsLengthCache, nil
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
			e.lessonsLengthCache = &length
			return &length, nil
		}
	}
}

func (e *implNotesExtractor) GetLessons(ctx context.Context) (*[]string, error) {
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

func (e *implNotesExtractor) GetSubjectsByLesson(ctx context.Context, indexOfLesson int) (*[]string, error) {
	e.logger.Debug(fmt.Sprintf("start get cards from lesson, by index: %d", indexOfLesson))
	select {
	case <-ctx.Done():
		{
			e.logger.Debug("context is done, leaving from 'GetCard'")
			return nil, ctx.Err()
		}
	default:
		{
			length, err := e.GetLessonsLength(ctx)
			if err != nil {
				return nil, err
			}

			if indexOfLesson < *length && indexOfLesson > -1 {
				page, err := e.gotoNotesPage(ctx)
				if err != nil {
					return nil, err
				}

				lessonLabels, err := e.GetLessons(ctx)
				if err != nil {
					return nil, err
				}

				lessonLabel := (*lessonLabels)[indexOfLesson]
				//This method click on navigation item and wait when page is load
				_, err = e.getWordlistNavigation(ctx, page)
				if err != nil {
					return nil, err
				}
				_, err = e.getNavigationItemWithInnerItems(ctx, page, lessonLabel)
				if err != nil {
					return nil, err
				}

				cardLocators, err := page.Locator(selector_for_flash_card_notes).All()
				if err != nil {
					e.logger.Error("failed get flashcards from page", slog.Any("err", err.Error()))
					return nil, err
				}

				e.logger.Debug("start getting subjects from card locators")
				subjects := make([]string, len(cardLocators))
				for i := range cardLocators {
					subject, err := e.getInnerTextFromChild(cardLocators[i], selector_for_subject_notes)
					if err != nil {
						return nil, err
					}
					subjects[i] = *subject
				}

				e.logger.Debug("end getting subjects from card locators")
				return &subjects, nil
			}

			return nil, errIndexOutOfRange
		}
	}
}

func (e implNotesExtractor) gotoNotesPage(ctx context.Context) (playwright.Page, error) {
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
			_, err = page.Goto(main_notes_page)
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

func (e *implNotesExtractor) getWordlistNavigation(ctx context.Context, page playwright.Page) (playwright.Locator, error) {
	return e.getNavigationItemWithInnerItems(ctx, page, "Wordlist")
}

func (e *implNotesExtractor) getNavigationItemWithInnerItems(ctx context.Context, page playwright.Page, navigationItemLabel string) (playwright.Locator, error) {
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
	err = navigationItemLocator.Locator(selector_for_navigation_item_notes).First().WaitFor()
	if err != nil {
		e.logger.Error("failed waiting open inner navigation items", slog.Any("err", err.Error()))
		return nil, err
	}
	return navigationItemLocator, nil
}

func (e *implNotesExtractor) getNavigationItemByLabel(ctx context.Context, page playwright.Page, label string) (playwright.Locator, error) {
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

			err := page.Locator(selector_for_navigation_item_notes).First().WaitFor()
			if err != nil {
				e.logger.Error("failed wait for loading navigation items", slog.Any("err", err.Error()))
				return nil, err
			}
			navigationItemLocators, err := page.Locator(selector_for_navigation_item_notes).All()
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

func (e *implNotesExtractor) getLessonLocators(ctx context.Context, page playwright.Page) (*[]playwright.Locator, error) {
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

			navigationItemLocators, err := wordListLocator.Locator(selector_for_navigation_item_notes).All()
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

func (e *implNotesExtractor) getInnerTextFromChild(parentLocator playwright.Locator, selector string) (*string, error) {
	childLocator := parentLocator.Locator(selector).First()
	innerText, err := childLocator.InnerText()
	if err != nil {
		e.logger.Error("failed get inner text from locator", slog.Any("err", err.Error()))
		return nil, err
	}
	return &innerText, nil
}
