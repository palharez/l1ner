# Optional audio identification

Use `l1ner identify` to find one MP3 by sound, even if its title and artist are missing or wrong. AcoustID setup is only needed for this command. `inspect` works offline, and `lookup`, `preview`, and `fix` use MusicBrainz's tag-based search without an AcoustID key. Setting the key makes it available to `identify`. Running the command is what starts the lookup.

## How AcoustID works

Chromaprint's `fpcalc` reads the MP3 locally and produces a fingerprint, a compact description of its sound. l1ner sends the fingerprint and the audio duration over HTTPS to AcoustID, which searches its fingerprint database for linked MusicBrainz recordings. The request uses an AcoustID application API key. The MP3, its path, and its tags stay on your machine. The hosted service is free for non-commercial use. [Chromaprint](https://acoustid.org/chromaprint) · [AcoustID web service](https://acoustid.org/webservice#lookup)

## Set up on Linux

1. Install `fpcalc` from your Linux distribution or download the official Linux binary for your CPU and place it on `PATH`. Run `fpcalc -version` to check the installation. [Chromaprint downloads](https://acoustid.org/chromaprint)
2. Open AcoustID's application registration page and sign in with your MusicBrainz or Google account. Return to the registration page after signing in if needed. [Register an AcoustID application](https://acoustid.org/new-application)
3. Enter `l1ner` as the application name, your version (use `devel` for a source build), and a contact email. The website field is optional and can point to the l1ner repository. Select **Register**. [AcoustID registration form source](https://github.com/acoustid/acoustid-server/blob/master/acoustid/web/templates/new-application.html)
4. Open `l1ner` under **Your Applications** and copy the **API key** from its details page. Use that application key for lookups. AcoustID also shows a separate user API key on your account page for submitting fingerprints. [AcoustID's explanation of both keys](https://acoustid.org/faq#why-is-my-api-key-not-working)
5. In the Bash shell where you run l1ner, paste the key at the hidden prompt and press Enter. The setting lasts for that shell session.

   ```bash
   read -r -s -p 'AcoustID application API key > ' L1NER_ACOUSTID_KEY
   printf '\n'
   export L1NER_ACOUSTID_KEY
   ./bin/l1ner identify music/song.mp3
   ```

   Run `unset L1NER_ACOUSTID_KEY` to clear the key from the current shell.

## Read the results

Each candidate shows a title, artist, MusicBrainz recording ID, and AcoustID score. Distinct recordings remain separate. Repeated hits for the same recording keep their highest score. The score ranks fingerprint hits, and several recordings can share a score. Review the candidates before using their metadata. The command leaves the MP3 byte for byte as it was. [Live validation](sample-validation.md#audio-identification)

## Troubleshooting

- **Missing key.** Set `L1NER_ACOUSTID_KEY` in the same shell that runs l1ner.
- **AcoustID rejects the request.** Check that you copied the key from the application page. The account's user key is for submissions.
- **Missing `fpcalc`.** Run `fpcalc -version` in the same shell and check that its directory is on `PATH`.
- **No candidates.** The lookup completed without a matching fingerprint linked to a MusicBrainz recording. If the MP3 has a title and artist, you can try `l1ner lookup <file.mp3>`.
- **Rate limit or service busy.** Wait before retrying. l1ner coordinates one AcoustID request per second across invocations using the same cache directory. The local calculation times out after 30 seconds and the request after 10.

A completed lookup exits `0`, including when there are no candidates. Input, configuration, service, and output errors exit `1`. Invalid arguments exit `2`. Ctrl+C exits `130`.
