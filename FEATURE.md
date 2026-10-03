## Feature: Add rule to prompt.
Add to promt rule thats subject should be in lowercase, except proper nouns

## Feature: Change generation flashcard for words and phrase.
Right now app has to scenarios generating content for flashcards: word and phrase. I want change this scenarious to 'only ai' and 'with dictionary'
Workflow for scenario with cambridge dictionary:
- try to find subject in cambridge dictionary. @extractor.cambridge.go
- if one one volumes missing, generate it by AI (paraphrase, examples, synonyms, transcription) @llm.client.go or by tts for pronunciation @tts.speech.go
- picture will be gotten from duckduckgo, and rate by ai @duckduckgo.image.go @llm.client.go

Workflow for scenario with ai:
- generate paraphrase, examples, synonyms, transcription by llm @llm.client.go
- generate pronunciation by tts client @tts.speech.go
- find picture from duckduckgo image search @duckduckgo.image.go
- rate picture by llm @llm.client.go


2. TUI interface for service
3. Integrate tts with espeak-ng
