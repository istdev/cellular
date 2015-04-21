package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	cell "github.com/wiless/cellular"

	"github.com/wiless/cellular/antenna"
	"github.com/wiless/cellular/deployment"
	"github.com/wiless/cellular/pathloss"
	"github.com/wiless/vlib"
)

var matlab *vlib.Matlab

var templateAAS *antenna.SettingAAS

type Point struct {
	X, Y float64
}

type LinkInfo struct {
	RxID              int
	NodeTypes         []string
	LinkGain          vlib.VectorF
	LinkGainNode      vlib.VectorI
	InterferenceLinks vlib.VectorF
}

var angles vlib.VectorF = vlib.VectorF{45, -45, -135, -45}
var singlecell deployment.DropSystem

func main() {
	matlab = vlib.NewMatlab("deployment")
	matlab.Silent = true
	matlab.Json = true

	seedvalue := time.Now().Unix()
	/// Setting to fixed seed
	seedvalue = 0
	rand.Seed(seedvalue)

	templateAAS = antenna.NewAAS()
	templateAAS.SetDefault()
	templateAAS.Omni = true
	// modelsett:=pathloss.NewModelSettingi()
	var model pathloss.PathLossModel
	model.ModelSetting.SetDefault()
	model.ModelSetting.Param[0] = 2
	DeployLayer1(&singlecell)

	singlecell.SetAllNodeProperty("BS", "AntennaType", 0)
	singlecell.SetAllNodeProperty("UE", "AntennaType", 1) /// Set All Pico to use antenna Type 1

	singlecell.SetAllNodeProperty("BS", "FreqGHz", vlib.VectorF{0.4, 0.5, 0.6, 0.7, 0.8}) /// Set All Pico to use antenna Type 0
	singlecell.SetAllNodeProperty("UE", "FreqGHz", vlib.VectorF{0.4, 0.5, 0.6, 0.7, 0.8}) /// Set All Pico to use antenna Type 0

	// lininfo := CalculatePathLoss(&singlecell, &model)

	rxids := singlecell.GetNodeIDs("UE")
	type MFNMetric []cell.LinkMetric
	MetricPerRx := make(map[int]MFNMetric)
	var AllMetrics MFNMetric
	wsystem := cell.NewWSystem()
	wsystem.BandwidthMHz = 10
	MaxCarriers := 1
	for _, rxid := range rxids {
		metrics := wsystem.EvaluteMetric(&singlecell, &model, rxid, myfunc)
		if len(metrics) > 1 {
			// log.Printf("%s[%d] Supports %d Carriers", "UE", rxid, len(metrics))
			MaxCarriers = int(math.Max(float64(MaxCarriers), float64(len(metrics))))
			// log.Printf("%s[%d] Links %#v ", "UE", rxid, metrics)
		}
		AllMetrics = append(AllMetrics, metrics...)
		MetricPerRx[rxid] = metrics
	}
	// vlib.SaveMapStructure2(MetricPerRx, "linkmetric.json", "UE", "LinkMetric", true)
	vlib.SaveStructure(AllMetrics, "linkmetric2.json", true)
	//Generate SINR values for CDF
	SINR := make(map[float64]vlib.VectorF)

	for _, metric := range MetricPerRx {
		for f := 0; f < len(metric); f++ {

			temp := SINR[metric[f].FreqInGHz]
			temp.AppendAtEnd(metric[f].BestSINR)
			SINR[metric[f].FreqInGHz] = temp
		}
	}
	matlab.Close()
	cnt := 0
	matlab = vlib.NewMatlab("sinrVal.m")
	for f, sinr := range SINR {
		log.Printf("\n F%d=%f \nSINR_%d= %v", cnt, f, cnt, sinr)
		str := fmt.Sprintf("sinr_%d", cnt)
		matlab.Export(str, sinr)
		cnt++
	}
	fmt.Println("\n")
	matlab.Command("cdfplot(sinr_0);hold all;cdfplot(sinr_1);cdfplot(sinr_2);legend('400','850','1800')")
	matlab.Close()
	fmt.Println("\n")
}

func DeployLayer1(system *deployment.DropSystem) {
	setting := system.GetSetting()
	if setting == nil {
		setting = deployment.NewDropSetting()
	}

	CellRadius := 500.0
	nUEPerCell := 2000
	nCells := 19
	AreaRadius := CellRadius
	setting.SetCoverage(deployment.CircularCoverage(AreaRadius))
	setting.AddNodeType(deployment.NodeType{Name: "BS", Hmin: 30.0, Hmax: 30.0, Count: nCells})
	setting.AddNodeType(deployment.NodeType{Name: "UE", Hmin: 1.1, Hmax: 1.1, Count: nUEPerCell * nCells})

	// setting.AddNodeType(waptype)
	/// You can save the settings of this deployment by uncommenting this line
	system.SetSetting(setting)
	system.Init()

	setting.SetTxNodeNames("BS")
	setting.SetRxNodeNames("UE")
	/// Drop BS Nodes
	{
		locations := deployment.HexGrid(system.NodeCount("BS"), vlib.FromCmplx(deployment.ORIGIN), CellRadius, 30)
		system.SetAllNodeLocation("BS", vlib.Location3DtoVecC(locations))
		// system.DropNodeType("BS")
		// find UE locations
		var uelocations vlib.VectorC
		for _, bsloc := range locations {
			// log.Printf("Deployed for cell %d ", indx)
			ulocation := deployment.HexRandU(bsloc.Cmplx(), CellRadius, nUEPerCell, 30)
			uelocations = append(uelocations, ulocation...)
		}
		system.SetAllNodeLocation("UE", uelocations)
	}

	vlib.SaveStructure(&system, "giridep.json", true)

}

func myfunc(nodeID int) antenna.SettingAAS {
	// atype := singlecell.Nodes[txnodeID]
	/// all nodeid same antenna
	return *templateAAS
}
