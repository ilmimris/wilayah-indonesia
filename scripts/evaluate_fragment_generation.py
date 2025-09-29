import json
import re
import string

def normalize(value: str) -> str:
    """
    Normalize a text value into a compact, canonical tokenized form.
    
    The function trims leading and trailing whitespace, limits input to the first 100 characters, lowercases the text, replaces punctuation and other non-word characters with spaces, collapses consecutive whitespace into single spaces, and returns at most the first 10 whitespace-separated tokens joined by single spaces.
    
    Parameters:
        value (str): Input string to normalize.
    
    Returns:
        str: The normalized string; returns an empty string if the trimmed input is empty.
    """
    value = value.strip()
    if not value:
        return ""

    if len(value) > 100:
        value = value[:100]

    # In Go, this is done with unicode.IsLetter, unicode.IsDigit, etc.
    # This is a simplification.
    value = value.lower()
    value = re.sub(r'[^\w\s]', ' ', value)
    value = re.sub(r'\s+', ' ', value).strip()

    tokens = value.split()
    if len(tokens) > 10:
        tokens = tokens[:10]
    return " ".join(tokens)

def candidate_fragments(query: str, word_combo_size: int) -> list[str]:
    """
    Generate a list of candidate fragment strings derived from a query for use in matching or search.
    
    The returned fragments are normalized (lowercased, punctuation removed, whitespace collapsed), unique, and ordered by descending length. For long queries (more than 100 characters) this returns up to the first five normalized words from the normalized prefix of the query. Otherwise the function extracts fragments from the whole query (including splits on common separators, individual words, and contiguous multi-word phrases up to `word_combo_size`), and returns up to five distinct fragments.
    
    Parameters:
        query (str): The input query text to derive fragments from.
        word_combo_size (int): Maximum number of words to combine when producing multi-word fragments.
    
    Returns:
        list[str]: A list of up to five normalized, unique fragment strings sorted by length (longest first).
    """
    if len(query) > 100:
        words = normalize(query[:50]).split()
        if len(words) > 5:
            words = words[:5]
        return words

    seen = set()
    
    def add(text: str):
        """
        Add a normalized, non-empty fragment to the enclosing `seen` set if it is not already present.
        
        Parameters:
            text (str): Candidate fragment text to normalize and add.
        """
        normalized = normalize(text)
        if not normalized:
            return
        if normalized in seen:
            return
        seen.add(normalized)

    add(query)

    separators = [",", "|", ";", "-", "/", "\\n"]
    for sep in separators:
        parts = query.split(sep)
        for part in parts:
            add(part)
            if len(seen) > 20:
                break
        if len(seen) > 20:
            break

    words = normalize(query).split()
    if words:
        for word in words:
            add(word)
            if len(seen) > 20:
                break
        
        if len(seen) <= 20:
            for size in range(2, word_combo_size + 1):
                if len(words) < size:
                    break
                for i in range(len(words) - size + 1):
                    add(" ".join(words[i:i+size]))
                    if len(seen) > 20:
                        break
                if len(seen) > 20:
                    break

    fragments = list(seen)
    fragments.sort(key=len, reverse=True)

    if len(fragments) > 5:
        fragments = fragments[:5]

    return fragments

def main():
    """
    Run a diagnostic that compares 2-word vs 3-word fragment generation for a sample of place-name queries and report any differences.
    
    Reads matcher_snapshot.json (falling back to an empty facets list on read/parse error), builds a representative set of queries from sampled facet entries plus known complex names, generates fragments using candidate_fragments with word sizes 2 and 3, and prints a summary of queries where 3-word fragments are missing from the 2-word results along with example cases and a recommendation.
    """
    try:
        with open('/Users/ilmimris/Proj/external/ilmimris/wilayah-indonesia/data/matcher_snapshot.json', 'r') as f:
            data = json.load(f)
    except (IOError, json.JSONDecodeError) as e:
        print(f"Error reading or parsing matcher_snapshot.json: {e}")
        # Create a dummy data structure if the file is missing or corrupt
        data = {'facets': []}


    facets = data['facets']
    
    # Create a representative sample of queries
    queries = []
    if facets:
        for i in range(0, len(facets), 1000): # Sample every 1000th entry
            facet = facets[i]
            if facet.get('Subdistrict'):
                queries.append(facet['Subdistrict'])
            if facet.get('District'):
                queries.append(facet['District'])
            if facet.get('City'):
                queries.append(facet['City'])
            if facet.get('Province'):
                queries.append(facet['Province'])
            
            # Add some multi-word queries
            if facet.get('District') and facet.get('City'):
                queries.append(f"{facet['District']} {facet['City']}")
            if facet.get('Subdistrict') and facet.get('District'):
                queries.append(f"{facet['Subdistrict']} {facet['District']}")

    # Add some known complex names
    queries.extend([
        "Jakarta Selatan",
        "Kota Administrasi Jakarta Selatan",
        "Kabupaten Bandung Barat",
        "Bandung Barat",
        "Gunung Kidul Yogyakarta",
        "DI Yogyakarta",
    ])

    print(f"Generated {len(queries)} test queries.")

    missed_fragments = {}

    for query in queries:
        fragments_2_word = candidate_fragments(query, 2)
        fragments_3_word = candidate_fragments(query, 3)

        missed = set(fragments_3_word) - set(fragments_2_word)
        if missed:
            missed_fragments[query] = list(missed)

    print("\n--- Analysis of Fragment Generation ---")
    if not missed_fragments:
        print("No important fragments were missed by using 2-word combinations instead of 3-word combinations.")
    else:
        print(f"Found {len(missed_fragments)} queries where 3-word combinations generated fragments missed by 2-word combinations.")
        print("These fragments might be important for search accuracy.")
        print("\nExample of missed fragments:")
        count = 0
        for query, fragments in missed_fragments.items():
            if count >= 5:
                break
            print(f"  Query: '{query}'")
            print(f"    Missed: {fragments}")
            count += 1
            
    print("\n--- Conclusion ---")
    if not missed_fragments:
        print("The current implementation of using 2-word combinations seems sufficient and the claim in OPTIMIZATIONS.md is likely valid.")
    else:
        print("The reduction from 3-word to 2-word combinations in `candidateFragments` can lead to missed search fragments.")
        print("This could negatively impact search accuracy for multi-word place names.")
        print("Recommendation: Revert the change in `internal/usecase/region/matcher/matcher.go` to use 3-word combinations, or make it configurable.")

if __name__ == "__main__":
    main()
