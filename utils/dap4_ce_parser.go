package utils

import (
	"log"
	"fmt"
	"strings"
	"regexp"
	"strconv"
	"math"
	"time"
)

type DapIdxSelector struct {
	Start   *int
	End     *int
	Step    *int
	IsRange bool
	IsAll   bool
}

type DapVarParam struct {
	Name string
	ValStart   *float64
	ValEnd     *float64
	IdxSelectors []*DapIdxSelector
	IsAxis     bool
}

type DapConstraints struct {
	VarParams []*DapVarParam
}

var varNameRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
func ParseDap4ConstraintExpr(ceStr string) (*DapConstraints, error) {

	selection := strings.Split(strings.TrimSpace(ceStr), "|")
	if len(selection) > 2 {
		return nil, fmt.Errorf("only a single filter expression is supported")
	}

	ce := &DapConstraints{}
	subset := strings.TrimSpace(selection[0])

	var filters string
	if len(selection) == 2 {
		filters = strings.TrimSpace(selection[1])	
	}

	var dataset string
	iDs := -1
	for i := 0; i < len(subset); i++ {
		if subset[i] == '{' {
			dataset = subset[:i]
			iDs = i
			break
		}
	}

	dataset = strings.TrimSpace(dataset)
	if iDs < 0 || len(dataset) == 0 {
		return nil, fmt.Errorf("dataset not found")
	}

	if subset[len(subset)-1] != '}' {
		return nil, fmt.Errorf("missing }")
	}

	varStr := subset[iDs+1:len(subset)-1]
	err := parseVariables(varStr, ce)
	if err != nil {
		return nil, err
	}

	err = parseFilters(filters, ce)
	if err != nil {
		return nil, err
	}

	varLookup := make(map[string]bool)
	for _, vp := range ce.VarParams {
		_, found := varLookup[vp.Name]
		if !found {
			varLookup[vp.Name] = true
		} else {
			return nil, fmt.Errorf("duplicated dimension variable: %v", vp.Name)
		}
	}

	dumpCE(ce)
	return ce, nil
}

func parseVariables(varStr string, ce *DapConstraints) error {
	for _, va := range strings.Split(varStr, ";") {
		va = strings.TrimSpace(va)
		if len(va) == 0 {
			continue
		}

		iVa := -1
		for iv := 0; iv < len(va); iv++ {
			if va[iv] == '[' {
				iVa = iv
				break
			}
		}

		varParam := &DapVarParam{}		
		if iVa < 0 {
			if !varNameRegex.MatchString(va) {
				return fmt.Errorf("invalid variable name: %v", va)
			}

			varParam.Name = va
			ce.VarParams = append(ce.VarParams, varParam)
			continue
		}

		varParam.Name = strings.TrimSpace(va[:iVa])
		if len(varParam.Name) == 0 {
			return fmt.Errorf("variable not found: %v", va)
		}

		if !varNameRegex.MatchString(varParam.Name) {
			return fmt.Errorf("invalid variable name: %v", varParam.Name)
		}

		if va[len(va)-1] != ']' {
			return fmt.Errorf("missing ]: %v", va)
		}
		varParam.IsAxis = true

		idxSel := va[iVa+1:len(va)-1]	
		selectors, err := parseVarSelectors(idxSel)
		if err != nil {
			return err
		}
		varParam.IdxSelectors = selectors

		ce.VarParams = append(ce.VarParams, varParam)
	}

	return nil

}

func parseVarSelectors(idxSel string) ([]*DapIdxSelector, error) {
	var selectors []*DapIdxSelector

	idxPartsRaw := strings.Split(idxSel, ",") 
	var idxParts []string
	for _, idxS := range idxPartsRaw {
		idxS = strings.TrimSpace(idxS)
		if len(idxS) == 0 {
			continue
		}
		idxParts = append(idxParts, idxS)
	}

	if len(idxParts) == 0 {
		selectors = append(selectors, &DapIdxSelector{IsRange: true, IsAll: true})
		return selectors, nil
	}

	for _, idxS := range idxParts {
		idxS = strings.TrimSpace(idxS)
		if len(idxS) == 0 {
			continue
		}

		sels := &DapIdxSelector{IsRange: true} 		

		selParts := strings.Split(idxS, ":")
		if len(selParts) > 3 {
			return selectors, fmt.Errorf("invalid selector: %v", idxS)
		}

		var selVals [3]int
		for iss, selStr := range selParts {
			selStr = strings.TrimSpace(selStr)
			nodata := len(selStr) == 0

			if !nodata {
				if len(selParts) == 1 {
					sels.IsRange = false
				}

				val64, err := strconv.ParseInt(selStr, 10, 64)
				if err != nil {
					return selectors, fmt.Errorf("invalid selector: %v", idxS)
				}
				selVals[iss] = int(val64)

				if iss == 0 {
					sels.Start = &selVals[iss]
				} else if iss == 1 {
					if len(selParts) == 3 {
						sels.Step = &selVals[iss]
					} else {
						sels.End = &selVals[iss]
					}
				} else if iss == 2 {
					sels.End = &selVals[iss]
				}
			}
		}
		selectors = append(selectors, sels)
	}

	return selectors, nil

}

func parseFilters(fltStr string, ce *DapConstraints) error {
	var relOps = map[string]int{">": 0, ">=": 0, "<": 1, "<=": 1, "=": 2}

	for _, flt := range strings.Split(fltStr, ",") {
		flt = strings.TrimSpace(flt)
		if len(flt) == 0 {
			continue
		}

		iRel := -1
		var relOp string
		for i := 0; i < len(flt); i++ {
			_, found := relOps[string(flt[i])]
			if found {
				iRel = i
				relOp = string(flt[i])
				break
			}
		}

		if iRel < 0 {
			return fmt.Errorf("invalid filter expression: %v", flt)
		}

		leftOp := strings.TrimSpace(flt[:iRel])
		if len(leftOp) == 0 {
			return fmt.Errorf("filter expression missing left op: %v", flt)
		}

		if iRel+1 < len(flt) && flt[iRel+1] == '=' {
			relOp = string(flt[iRel:iRel+1])
			iRel++
		}

		iRel2 := -1
		var relOp2 string
		for i := iRel+1; i < len(flt); i++ {
			_, found := relOps[string(flt[i])]
			if found {
				iRel2 = i
				relOp2 = string(flt[i]) 
				break
			}
		}

		var midOp, rightOp string
		if iRel2 < 0 {
			if iRel+1 >= len(flt) {
				return fmt.Errorf("invalid filter expression: %v", flt)
			}
			rightOp = strings.TrimSpace(flt[iRel+1:])
		} else {
			midOp = strings.TrimSpace(flt[iRel+1:iRel2])
			if iRel2+1 < len(flt) && flt[iRel2+1] == '=' {
				relOp2 = string(flt[iRel2:iRel2+1]) 
				iRel2++
			}

			if iRel2+1 >= len(flt) {
				return fmt.Errorf("invalid filter expression: %v", flt)
			}
			rightOp = strings.TrimSpace(flt[iRel2+1:])
		}

		log.Printf("%s, %s, %s", leftOp, midOp, rightOp)

		varParam := &DapVarParam{IsAxis: true}		
		if len(midOp) == 0 {
			if !varNameRegex.MatchString(leftOp) {
				return fmt.Errorf("invalid variable name for the left op: %v", leftOp)
			}
			varParam.Name = leftOp

			fVal, err := parseEndpoint(rightOp)
			if err != nil {
				return err
			}

			lowerVal := math.SmallestNonzeroFloat64
			upperVal := math.MaxFloat64

			if relOps[relOp] == 0 {
				upperVal = fVal
				varParam.ValStart = &lowerVal
				varParam.ValEnd = &upperVal
			} else if relOps[relOp] == 1 {
				lowerVal = fVal
				varParam.ValStart = &lowerVal
				varParam.ValEnd = &upperVal
			} else if relOps[relOp] == 2 {
				lowerVal = fVal
				varParam.ValStart = &lowerVal
			} else {
				return fmt.Errorf("unknown rel op code: %v", relOp)
			}

			if lowerVal > upperVal {
				return fmt.Errorf("lower endpoint greater than upper endpoint: %v", flt)
			}
		} else {
			if relOps[relOp] != relOps[relOp2] {
				return fmt.Errorf("invalid filter expression: %v", flt)
			}

			isValid := relOps[relOp] == 0 || relOps[relOp] == 1
			if !isValid {
				return fmt.Errorf("invalid filter expression: %v", flt)
			}

			if !varNameRegex.MatchString(midOp) {
				return fmt.Errorf("invalid variable name for the middle op: %v", midOp)
			}
			varParam.Name = midOp

			fVal1, err1 := parseEndpoint(leftOp)
			if err1 != nil {
				return err1
			}

			fVal2, err2 := parseEndpoint(rightOp)
			if err2 != nil {
				return err2
			}

			if relOps[relOp] == 1 {
				tmp := fVal1
				fVal1 = fVal2
				fVal2 = tmp
			}

			lowerVal := fVal1
			upperVal := fVal2

			if lowerVal > upperVal {
				return fmt.Errorf("lower endpoint greater than upper endpoint: %v", flt)
			}

			varParam.ValStart = &lowerVal
			varParam.ValEnd = &upperVal
		}

		ce.VarParams = append(ce.VarParams, varParam)
	}

	return nil
}

func parseEndpoint(valStr string) (float64, error) {
	fVal, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		tVal, errTime := time.Parse(ISOFormat, valStr)
		if errTime != nil {
			return 0, fmt.Errorf("invalid endpoint: %v", valStr)
		}
		fVal = float64(tVal.Unix())
	}

	return fVal, nil
}

func dumpCE(ce *DapConstraints) {
	for _, vp := range ce.VarParams {
		vpStr := vp.Name + ", valStart="
		if vp.ValStart != nil {
			vpStr += fmt.Sprintf("%.8e", *vp.ValStart)
		} else {
			vpStr += "nil"
		}

		vpStr += ", valEnd="
		if vp.ValEnd != nil {
			vpStr += fmt.Sprintf("%.8e", *vp.ValEnd)
		} else {
			vpStr += "nil"
		}

		vpStr += ", sels="

		for iss, sel := range vp.IdxSelectors {
			if sel.IsAll {
				vpStr += "all"
			} else {
				if !sel.IsRange {
					vpStr += fmt.Sprintf("singleton:%d", *sel.Start)
				} else {
					if sel.Start != nil {
						vpStr += fmt.Sprintf("start:%d", *sel.Start)
					}

					if sel.Step != nil {
						vpStr += fmt.Sprintf(",step:%d", *sel.Step)
					}

					if sel.End != nil {
						vpStr += fmt.Sprintf(",end:%d", *sel.End)
					}
				}
			}

			if iss < len(vp.IdxSelectors) - 1 {
				vpStr += "; "
			}
		}

		vpStr += ", isAxis="
		if vp.IsAxis {
			vpStr += "true"
		} else {
			vpStr += "false"
		}

		log.Printf("%s", vpStr)
	}
}

