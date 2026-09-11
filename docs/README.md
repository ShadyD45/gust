# Documentation source

This directory is the source for the **gust** documentation site, built with [Just the Docs](https://just-the-docs.github.io/just-the-docs/) and published via GitHub Pages.

- **Live site:** https://shadyd45.github.io/gust/
- **Public sections:** Usage, Benchmarks, Extending, Architecture
- **Roadmap:** GitHub issues [#1](https://github.com/ShadyD45/gust/issues/1)–[#5](https://github.com/ShadyD45/gust/issues/5) (maintainers track implementation there)

## Preview locally

```bash
cd docs
bundle install
bundle exec jekyll serve
```

Open http://127.0.0.1:4000/gust/

## Publish

Push to `main`. The [jekyll-gh-pages](../.github/workflows/jekyll-gh-pages.yml) workflow builds and deploys. If Settings → Pages still asks for a source, choose **GitHub Actions**.
