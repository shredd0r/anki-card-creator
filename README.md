# Anki card creator

This tools helps you create flashcards in anki for improving your English vocablurary. For template is used [Maple Template X](https://github.com/liuzikai/Maple-Anki-Template) with small changes.

Flashcard has this fields:
* Subject - word, phrase, idiom which you want learn;
* Pronunciation - sound file with pronunciation word;
* Transcription;
* Description - describe subject in front side flashcard;
* Picture - describe subject in both sides flashcard;
* Examples - examples how you can use this subject; 

Information for flashcards is taken from: 
* [Cambridge dictionary](https://dictionary.cambridge.org/), main source, because this volumes already approved by humans;
* [Gemini API](https://gemini.google.com), used for generate volumes if it not found in dictionary, and rating picture for flashcard;
* Google image, for find pictures for flashcard

## Features
As you know, for some fields used LLM. This creator support ollama apies, you can run local LLM by ollama and use it for generating flashcards.

## Configuration:
You can use configuration yaml file or transmit volumes by run parameters
* ***debugging*** - (type: ***bool***) debugging mode, turn on showing browser window created by playwright, debug logging;
* ***queue.size*** - (type: ***uint***) ***default: 10***, size of flashcard creating at the same time;
* ***gemini*** - indicates that`s for generate volumes will be used Gemini;  
* ***gemini.token*** - (type: ***string***) developing token created in your developer account in Google;
* ***ollama*** - indicates that`s for generate volumes will be used your local LLM runned in Ollama server;
* ***ollama.host*** - (type: ***string***) ***default: 127.0.0.1***, host to your ollama server;
* ***ollama.port*** - (type: ***uint***) ***default: 11434***, port to your ollama server;
* ***ollama.model*** - (type: ***string***) model name started on your ollama server, which will be used for generate volumes;
* ***picture.min-rating*** - (type: ***uint***) ***default: 7*** can be between 1 - 10, minimal rating for accept picture for flashcard
* ***picture.count-searches*** - (type: ***uint***) ***count-searches: 3*** count of searches and rate images for flashcard;
* ***picture.ignore*** - skip selection picture if during searing and rate gets error;
* ***config-file*** - path to config file with parameters;  

Example run:<br>
```bash 
    anki-card-creator --batch.size 5 --gemini --gemini.token token123 --picture.ignore
    
    // or
    
    anki-card-creator --ollama --ollama.port 8080 --ollama.model gemma --picture.count-searches 5

    // or

    anki-card-creator --config-file path-to-config.yaml
```

# TODO
* add supporting self-hosted llms
* add handling for gemini overload error
* handling critical (stop processing) / non-critical error