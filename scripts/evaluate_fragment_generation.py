
import json
import re
import string

def normalize(value: str) -> str:
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
    if len(query) > 100:
        words = normalize(query[:50]).split()
        if len(words) > 5:
            words = words[:5]
        return words

    seen = set()
    
    def add(text: str):
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
