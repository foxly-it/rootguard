package unbound

type Preset struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	BestFor     string   `json:"best_for"`
	Settings    Settings `json:"settings"`
}

type Recommendation struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Field       string `json:"field,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
}

type Advice struct {
	Status          string           `json:"status"`
	Recommendations []Recommendation `json:"recommendations"`
}

func Presets() []Preset {
	balanced := DefaultSettings()
	privacy := DefaultSettings()
	privacy.CacheMaxTTL = 43200
	resilience := DefaultSettings()
	resilience.CacheMinTTL = 60
	resilience.CacheMaxTTL = 172800
	performance := DefaultSettings()
	performance.CacheMinTTL = 300
	performance.CacheMaxTTL = 172800
	performance.Threads = 4
	performance.ResourceProfile = resourceProfileLarge

	return []Preset{
		{ID: "balanced", Name: "Ausgewogen", Description: "Sichere Standardwerte mit guter Aktualität und Cache-Effizienz.", BestFor: "Die meisten Heim- und kleinen Firmennetze", Settings: balanced},
		{ID: "privacy", Name: "Datenschutz", Description: "Kürzere maximale Cache-Dauer bei aktivierter QNAME-Minimierung und DNS-Verfügbarkeit.", BestFor: "Datenschutzorientierte Netze mit aktuellen Antworten", Settings: privacy},
		{ID: "resilience", Name: "Hohe Verfügbarkeit", Description: "Längere Cache-Nutzung und Serve Expired für vorübergehende externe DNS-Störungen.", BestFor: "Netze, in denen DNS-Ausfallsicherheit Vorrang hat", Settings: resilience},
		{ID: "performance", Name: "Performance", Description: "Effizienter Cache und zusätzliche Resolver-Threads für leistungsfähigere Hosts.", BestFor: "Größere Netze und Hosts mit mindestens vier CPU-Kernen", Settings: performance},
	}
}

func Advise(settings Settings) (Advice, error) {
	if err := settings.Validate(); err != nil {
		return Advice{}, err
	}
	recommendations := make([]Recommendation, 0, 6)
	add := func(id, severity, field, title, description, suggestion string) {
		recommendations = append(recommendations, Recommendation{ID: id, Severity: severity, Field: field, Title: title, Description: description, Suggestion: suggestion})
	}

	if !settings.QnameMinimisation {
		add("enable-qname-minimisation", "warning", "qname_minimisation", "Mehr Anfragedaten als notwendig", "Ohne QNAME-Minimierung erhalten übergeordnete Nameserver den vollständigen angefragten Namen.", "QNAME-Minimierung für einen datensparsameren rekursiven Betrieb aktivieren.")
	}
	if !settings.Prefetch {
		add("enable-prefetch", "recommendation", "prefetch", "Häufige Antworten können aus dem Cache fallen", "Prefetch erneuert häufig verwendete Einträge kurz vor ihrem Ablauf und reduziert wahrnehmbare Latenz.", "Prefetch aktivieren, sofern minimale zusätzliche Hintergrundabfragen akzeptabel sind.")
	}
	if !settings.ServeExpired {
		add("enable-serve-expired", "warning", "serve_expired", "Weniger widerstandsfähig bei externen Störungen", "Bereits bekannte Domains können bei einer vorübergehenden Störung autoritativer Nameserver nicht weiter beantwortet werden.", "Serve Expired für Heim- und Unternehmensnetze aktivieren.")
	}
	if settings.CacheMinTTL > 300 {
		add("lower-cache-min-ttl", "warning", "cache_min_ttl", "Minimum TTL erzwingt lange veraltete Einträge", "Ein hoher Mindestwert überschreibt kürzere TTL-Vorgaben der Domainbetreiber und kann Änderungen verzögern.", "Minimum TTL auf höchstens 300 Sekunden reduzieren.")
	}
	if settings.CacheMaxTTL < 3600 {
		add("raise-cache-max-ttl", "recommendation", "cache_max_ttl", "Cache wird sehr früh verworfen", "Eine maximale TTL unter einer Stunde erhöht externe DNS-Abfragen und reduziert den Cache-Nutzen.", "Maximum TTL auf mindestens 3600 Sekunden erhöhen.")
	}
	if settings.CacheMaxTTL > 172800 {
		add("lower-cache-max-ttl", "warning", "cache_max_ttl", "Sehr lange maximale Cache-Dauer", "Antworten können trotz längerer TTL mehrere Tage im Cache verbleiben und Änderungen später sichtbar werden.", "Maximum TTL auf höchstens 172800 Sekunden begrenzen.")
	}
	if settings.Threads == 1 {
		add("increase-threads", "recommendation", "threads", "Nur ein Resolver-Thread", "Ein einzelner Thread kann bei mehreren gleichzeitig aktiven Clients zum Engpass werden.", "Auf Hosts mit mehreren CPU-Kernen mindestens zwei Threads verwenden.")
	}
	if settings.Threads > 8 {
		add("review-threads", "warning", "threads", "Hohe Anzahl Resolver-Threads", "Viele Threads erhöhen Speicherbedarf und Kontextwechsel und helfen auf kleineren Hosts nicht.", "Thread-Anzahl an die tatsächlich verfügbaren CPU-Kerne anpassen.")
	}

	status := "optimized"
	for _, recommendation := range recommendations {
		if recommendation.Severity == "warning" {
			status = "review"
			break
		}
		status = "suggestions"
	}
	if len(recommendations) == 0 {
		add("configuration-looks-good", "success", "", "Konfiguration ist ausgewogen", "RootGuard hat für die verwalteten Werte keine problematische Kombination erkannt.", "Änderungen weiterhin zuerst über die Vorschau prüfen.")
	}
	return Advice{Status: status, Recommendations: recommendations}, nil
}
