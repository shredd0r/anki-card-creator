1. File @content_test.go. Add testcases for checking count searching pictures. Cases which need to add:
    - llmProvider.RatePicture returns n - 1 times rating less than configured and n time great than configured.
    - llmProvider.RatePicture returns n time rating less than configured
    - llmProvider.RatePicture return first time rating equal configured rating
    - llmProvider.RatePicture method return error
    - googleImageProvider return error
    - googleImageProvider return n - 1 times not suitable picture, last time return error
2. Subject in flashcard should be in lowercase
3. TUI interface for service
