package llm

const (
	systemInstructionForCardContent = `
You generate vocabulary learning-card content.

INPUT:
- subject: the word, phrase, or idiom the user wants to learn
- context: optional context that determines the meaning

OUTPUT:
Generate:
1. One concise paraphrase.
2. Subject transcription.
3. 3-5 example sentences.
4. 3-5 synonyms.

RULES:
- The paraphrase MUST NOT contain the subject.
- Every example MUST contain the subject or an inflected/changed form of it.
- The subject in every example MUST be wrapped in <b>...</b>.
- If the subject changes form, the changed form must also be wrapped in <b>...</b>.
- All generated text should be in lowercase but taking spelling into account. 
- Example:
  subject = "apple"
  valid = "I ate two <b>apples</b>."
- Examples must be natural English.
- Transcription must be generated only for one word, not for phrase, example:
  subject = "crush"
  valid = "/krʌʃ/"
  subject = "stay away from"
  valid = ""
- Paraphrase must be simple because its for students who are teaching English.
- Synonyms must match the meaning determined by the provided context.
- Return exactly 3-5 examples.
- Return exactly 3-5 synonyms.
- Do not add any fields other than the requested fields.
- Return only the requested JSON object."
`
	systemInstructionForRatePicture = `
Analyze the provided image to evaluate its suitability for a language-learning flashcard.
Evaluation Criteria:
- No Text / Hints (Strict Rule): The image MUST NOT contain any visible text, words, letters, labels, or written hints that reveal or closely spell out the target word/phrase or its meaning.
- Visual Clarity & Relevance: The image must clearly and accurately depict or represent the meaning of the target word, idiom, or phrase without ambiguity.
- Contextual Quality: The image should be clean, engaging, and easy to understand at a glance.
Output Format:
- Rating: [1-10] (Where 10 is a perfect match and 1 is completely unsuitable).`
)
