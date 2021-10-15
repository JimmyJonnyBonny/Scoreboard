package main

import (
	"log"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/DSU-DefSec/DWAYNE-INATOR-5000/checks"
	"github.com/pkg/errors"
)

func Score(m *config) {
	err := checkConfig(dwConf)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "illegal config"))
	}

	var record TeamRecord
	res := db.Debug().Limit(1).Order("time desc").Find(&record)
	if res.Error == nil {
		roundNumber = record.Round + 1
	} else {
		roundNumber = 1
	}

	rand.Seed(time.Now().UnixNano())
	// checkList = append(checkList, m.Web...)
	//mux := &sync.Mutex{}

	// Build initial PCR table
	constructPCRState()

	for {
		debugPrint("===================================")
		debugPrint("[SCORE] round", roundNumber)

		allTeamsWg := &sync.WaitGroup{}
		for _, t := range m.Team {
			allTeamsWg.Add(1)

			go func(team TeamData) {

				wg := &sync.WaitGroup{}
				resChan := make(chan checks.Result)

				debugPrint("team going into teamrecord is", team)
				newRecord := TeamRecord{
					Time:   time.Now().In(loc),
					TeamID: team.ID,
					Team:   team,
					Round:  roundNumber,
				}

				for _, b := range m.Box {
					for _, check := range b.CheckList {
						wg.Add(1)
						log.Println("running check for", team.Name, check)
						go checks.RunCheck(team.ID, team.IP, b.IP, b.Name, check, wg, resChan)
					}
				}

				done := make(chan struct{})
				go func() {
					wg.Wait()
					close(done)
				}()

				// team record
				doneSwitch := false
				for {
					select {
					case res := <-resChan:
						resEntry := ResultEntry{
							Time:   time.Now(),
							TeamID: team.ID,
							Round:  roundNumber,
							Result: checks.Result{
								Name:   res.Name,
								Status: res.Status,
								Error:  res.Error,
								Debug:  res.Debug,
								IP:     res.IP,
								Box:    res.Box,
							},
						}
						newRecord.Results = append(newRecord.Results, resEntry)
					case <-done:
						debugPrint("[SCORE] checks for team", team.Name, "are done")
						doneSwitch = true
					}
					if doneSwitch {
						break
					}
				}
				teamMutex.Lock()
				recordsStaging = append(recordsStaging, newRecord)
				teamMutex.Unlock()
				allTeamsWg.Done()
			}(t)
		}
		allTeamsWg.Wait()

		// Process all team records
		teamMutex.Lock()
		for _, rec := range recordsStaging {
			processNewRecord(&rec)
		}

		// Calculate persist points
		if dwConf.Persists {
			calculatePersists()
			// Reset persists
			persistHits = make(map[uint]map[string][]uint)
		}
		teamMutex.Unlock()

		// Next round!
		roundNumber++

		// Build PCR state before sleep.
		// We want submitted PCRs to miss at least one check round.
		debugPrint("[PCR] constructing PCR state")
		constructPCRState()

		jitter := time.Duration(0)
		if dwConf.Jitter != 0 {
			jitter = time.Duration(time.Duration(rand.Intn(dwConf.Jitter+1)) * time.Second)
		}
		debugPrint("[SCORE] sleeping for", dwConf.Delay, "with jitter", jitter)
		time.Sleep((time.Duration(dwConf.Delay) * time.Second) + jitter)
	}
}

func processNewRecord(rec *TeamRecord) {
	var currentRec TeamRecord

	result := db.Limit(1).Preload("Results").Order("time desc").Find(&currentRec, "team_id = ?", rec.Team.ID)
	if result.Error != nil {
		errorPrint(result.Error)
		return
	}

	// Calculate service and SLA values
	for _, res := range rec.Results {
		var slaRecord SLA
		result = db.First(&slaRecord, "team_id = ? and check_name = ?", rec.TeamID, res.Name)
		if result.Error != nil {
			slaRecord = SLA{
				TeamID:    rec.TeamID,
				CheckName: res.Name,
			}
		}
		if !res.Status {
			debugPrint("incrementing sla counter")
			slaRecord.Counter++
			if slaRecord.Counter >= dwConf.SlaThreshold {
				rec.SlaViolations++
				slaRecord.Violations++
				slaRecord.Counter = 0
			}
		} else {
			slaRecord.Counter = 0
			// If persists, everyone gets 1/N points
			if dwConf.Persists {

				rec.ServicePoints += dwConf.ServicePoints


			} else {
				rec.ServicePoints += dwConf.ServicePoints

			}
		}
		if result = db.Save(&slaRecord); result.Error != nil {
			errorPrint(result.Error)
			return
		}
	}
	// Add carry-over points
	rec.RedTeamPoints = currentRec.RedTeamPoints
	rec.InjectPoints = currentRec.InjectPoints
	rec.SlaViolations += currentRec.SlaViolations
	rec.PersistPoints += currentRec.PersistPoints
	rec.ServicePoints += currentRec.ServicePoints

	db.Create(&rec)

}

func calculatePersists() {
	var records []TeamRecord
	res := db.Find(&records)
	if res.Error != nil {
		errorPrint(res.Error)
		return
	}

	// Sort by team ID.
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].TeamID < records[j].TeamID
	})

	// I giveth points, I taketh points
	for team, boxes := range persistHits {
		for box, persists := range boxes {
			if len(persists) > 0 {
				// Get box points to split up.
				victim := records[team-1]
				debugPrint(box)
				debugPrint(victim)
			}
		}
	}
}


/*
func percentChangedCreds() map[string]float {
	// get all usernames
	// for each team, see which % of creds exist in pcritems
}
*/
