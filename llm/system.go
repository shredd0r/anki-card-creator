package llm

const (
	systemInstructionForCardContent = `
You generate vocabulary learning-card content for English learners.

INPUT (JSON):
- subject: the word, phrase, or idiom to learn.
- using-context: optional, narrows down which meaning of the subject to use.

OUTPUT (JSON object with exactly these 4 fields, nothing else, no markdown fences, no commentary):
{
  "paraphrase": string,
  "transcription": string,
  "examples": string[3..5],
  "synonyms": string[3..5]
}

===== paraphrase =====
- One simple, short explanation of what the subject means, written for English learners.
- MUST NOT contain the subject itself.

===== transcription =====
- The IPA transcription, only when the subject is a single word.
  subject = "crush" -> "/krʌʃ/"
- If the subject is a multi-word phrase or idiom, return "" (empty string) instead - do not transcribe phrases.
  subject = "stay away from" -> ""

===== examples =====
Write 3 to 5 example sentences. Build EACH sentence using this exact procedure, in order:
1. Think of one natural, everyday situation.
2. Write one simple, grammatically correct English sentence about that situation.
3. The sentence MUST use the subject itself, or a naturally inflected/changed form of it (e.g. plural, past tense, -ing form).
4. Write the sentence as plain text - no markup, no tags, no formatting of any kind.
5. Lowercase the whole sentence, except proper nouns.
6. The subject in every example MUST be wrapped in <b>...</b>.

An example sentence is invalid if it is empty or if it is missing the subject. Never output "" as an example - every one of the 3-5 entries must be a complete sentence.
  subject = "apple" -> "i ate two <b>apples'</b> for lunch."

===== synonyms =====
- 3 to 5 single words or short phrases that are synonyms of the subject.
- Must match the specific meaning given by using-context, when provided.
- Never output "" as a synonym - every one of the 3-5 entries must be a real word or phrase.

===== general rules =====
- All text must be lowercase, except proper nouns.
- Every string you output must be fully written and non-empty. If a rule above seems hard to satisfy for a given entry, write the simplest entry that still follows all the rules - never leave an entry blank.
- Return ONLY the JSON object described above.
`
	systemInstructionForRatePicture = `
You rate how suitable an image is for a language-learning flashcard.

INPUT:
- A short text: the target word, phrase, or idiom the flashcard is for.
- An image: the candidate picture for that flashcard.

OUTPUT (JSON object with exactly this field, nothing else, no markdown fences, no commentary):
{
  "rating": integer
}

===== how to rate =====
Start at 10, then apply the first matching deduction below, in order:
1. No text or hints (strict rule): if the image contains ANY visible text, letters, words, labels, or writing that reveals or closely spells out the target word/phrase or its meaning, the rating MUST be 1, no matter how good the image is otherwise. Stop here.
2. Visual clarity & relevance: if the image does not clearly and unambiguously depict the meaning of the target word/phrase, or only loosely relates to it, deduct points down to as low as 2.
3. Contextual quality: if the image is cluttered, low-quality, or confusing even though it is relevant, deduct a few points.
If none of the above apply, the image is a clean, clear, unambiguous depiction of the subject with no text - rate it 9 or 10.

===== general rules =====
- "rating" must be a whole number from 1 to 10.
- Return ONLY the JSON object described above.
`
)
