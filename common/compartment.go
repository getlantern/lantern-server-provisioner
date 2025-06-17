package common

import (
	"fmt"
	"time"
)

// CompartmentEntry represents an entry in a compartment, such as a project with associated cloud locations
type CompartmentEntry struct {
	ID        string          // Unique identifier for the entry (e.g., project ID)
	Locations []CloudLocation // List of locations associated with the entry
}

// Compartment represents a compartment in the cloud provider (billing account)
type Compartment struct {
	ID      string             // Unique identifier for the compartment
	Name    string             // Display name of the compartment (e.g., billing account name)
	Entries []CompartmentEntry // List of entries in the compartment (project IDs, etc.)
}

// some helper functions to extract IDs and names from compartments and entries

func CompartmentNames(compartments []Compartment) []string {
	ids := make([]string, len(compartments))
	for i, c := range compartments {
		ids[i] = c.Name
	}
	return ids
}

func CompartmentEntryIDs(entries []CompartmentEntry) []string {
	ids := make([]string, len(entries))
	for i, e := range entries {
		ids[i] = e.ID
	}
	return ids
}

func CompartmentEntryLocations(entry *CompartmentEntry) []string {
	locations := make([]string, 0)
	for _, e := range entry.Locations {
		loc := e.GetLocation()
		if loc == nil {
			continue // skip if location is nil
		}
		locations = append(locations, fmt.Sprintf("%s - %s [%s]", e.GetID(), loc.ID, loc.CountryCode))
	}
	return locations
}

func CompartmentLocationByIdentifier(entries []CloudLocation, id string) CloudLocation {
	for _, e := range entries {
		loc := e.GetLocation()
		if id == fmt.Sprintf("%s - %s [%s]", e.GetID(), loc.ID, loc.CountryCode) {
			return e
		}
	}
	return nil
}

func CompartmentByName(compartments []Compartment, name string) *Compartment {
	for _, c := range compartments {
		if c.Name == name {
			return &c
		}
	}
	return nil
}

func CompartmentEntryByID(entries []CompartmentEntry, id string) *CompartmentEntry {
	for _, e := range entries {
		if e.ID == id {
			return &e
		}
	}
	return nil
}

func MakeInstanceName() string {
	now := time.Now().UTC()
	return fmt.Sprintf("lantern-%s-%s", now.Format("20060102"), now.Format("150405"))
}
