package api

import "github.com/gin-gonic/gin"

// DownloadImportTemplate serves a CSV template for asset import.
// GET /api/v1/assets/import-template
func (s *APIServer) DownloadImportTemplate(c *gin.Context) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=asset-import-template.csv")

	// BOM for Excel UTF-8 recognition
	bom := "\xEF\xBB\xBF"

	header := "asset_tag,name,type,sub_type,status,bia_level,vendor,model,serial_number,property_number,control_number,ip_address,location,rack,tags,bmc_ip,bmc_type,bmc_firmware\n"
	example := "SRV-001,Production Server 01,server,rack_mount,operational,important,Dell,PowerEdge R750,SN-EXAMPLE-001,PN-001,CN-001,10.0.1.100,Taipei DC,Rack-A01,\"production,critical\",10.0.100.5,ilo,iLO 5 v2.72\n"

	c.String(200, bom+header+example)
}
