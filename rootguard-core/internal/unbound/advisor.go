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
	if !settings.PrefetchKey {
		add("enable-prefetch-key", "recommendation", "prefetch_key", "DNSSEC-Schlüssel werden erst bei Bedarf geladen", "Ohne Prefetch Key beginnt das Laden eines DNSKEY erst später im Validierungsablauf und kann die erste Antwort einer signierten Zone verzögern.", "Prefetch Key aktivieren; bei knappen CPU-Ressourcen lässt es sich gezielt deaktivieren.")
	}
	if !settings.AggressiveNSEC {
		add("enable-aggressive-nsec", "recommendation", "aggressive_nsec", "Validierte Negativantworten werden nicht wiederverwendet", "Ohne aggressives NSEC-Caching fragt Unbound häufiger nach nicht vorhandenen Namen, obwohl deren Nichtexistenz bereits DNSSEC-validiert ableitbar ist.", "Aggressives NSEC aktivieren; nur zur gezielten Diagnose auffälliger DNSSEC-Zonen vorübergehend deaktivieren.")
	}
	if settings.EDNSBufferSize < 1232 {
		add("raise-edns-buffer-size", "recommendation", "edns_buffer_size", "Kleiner EDNS-Puffer erzeugt mehr TCP-Rückfälle", "Ein EDNS-Puffer unter 1.232 Byte vermeidet Fragmentierung besonders streng, kann aber größere DNS- und DNSSEC-Antworten häufiger auf TCP verlagern.", "1.232 Byte als ausgewogenen DNS-Flag-Day-Standard verwenden, sofern der Netzwerkpfad keinen kleineren Wert erfordert.")
	}
	if settings.EDNSBufferSize > 1232 {
		add("lower-edns-buffer-size", "warning", "edns_buffer_size", "Große UDP-Antworten können fragmentieren", "Ein EDNS-Puffer über 1.232 Byte erhöht auf IPv6- und Tunnelpfaden das Risiko fragmentierter oder verworfener DNS-Antworten.", "EDNS-Puffer auf den sicheren Standardwert 1.232 Byte reduzieren.")
	}
	if settings.LogVerbosity == 0 {
		add("enable-operational-logging", "recommendation", "log_verbosity", "Nur Fehler werden protokolliert", "Die datensparsame Stufe 0 blendet auch allgemeine Betriebsinformationen aus und kann die Ursachenanalyse erschweren.", "Stufe 1 für Betriebsereignisse ohne dauerhafte Query- oder Reply-Protokollierung verwenden.")
	}
	if !settings.ServeExpired {
		add("enable-serve-expired", "warning", "serve_expired", "Weniger widerstandsfähig bei externen Störungen", "Bereits bekannte Domains können bei einer vorübergehenden Störung autoritativer Nameserver nicht weiter beantwortet werden.", "Serve Expired für Heim- und Unternehmensnetze aktivieren.")
	}
	if settings.ServeExpired && settings.ServeExpiredClientTimeout == 0 {
		add("delay-stale-answer", "recommendation", "serve_expired_client_timeout", "Abgelaufene Antworten werden sofort bevorzugt", "Bei 0 Millisekunden antwortet Unbound direkt aus dem abgelaufenen Cache, ohne zuerst eine frische Auflösung zu versuchen.", "Für RFC-8767-Fallbackverhalten 1.800 Millisekunden verwenden.")
	}
	if settings.ServeExpired && settings.ServeExpiredClientTimeout > 2500 {
		add("lower-stale-timeout", "warning", "serve_expired_client_timeout", "Lange Wartezeit vor der Notfallantwort", "Clients können ihre Anfrage abbrechen, bevor Unbound auf eine zulässige abgelaufene Antwort zurückfällt.", "Die Wartezeit auf höchstens 2.500 Millisekunden begrenzen.")
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
